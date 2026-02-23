from __future__ import annotations

import pathlib
import re
import subprocess
from typing import Dict, Tuple

ROOT = pathlib.Path(__file__).resolve().parents[2]  # /home/sam/locallife
SQL_PATH = ROOT / "locallife" / "db" / "migration" / "000136_update_agreements_v1_2_0_legal.up.sql"
OUT_DIR = ROOT / "legal_exports" / "agreements_v1_2_0"

TARGETS = {
    "MERCHANT_AGREEMENT": "商户入驻及数字化服务协议",
    "USER_AGREEMENT": "平台用户服务协议",
    "CONSUMER_RIGHTS": "消费者权益保障及纠纷处理规则",
}

HTML_SHELL = """<!doctype html>
<html lang=\"zh-CN\">
<head>
  <meta charset=\"utf-8\" />
  <meta name=\"viewport\" content=\"width=device-width, initial-scale=1\" />
  <title>{title}</title>
  <style>
    @page {{ size: A4; margin: 18mm 16mm; }}
    html, body {{ color: #111; }}
    body {{
      font-family: "Noto Sans CJK SC", "Noto Sans SC", "Microsoft YaHei", "PingFang SC", "Hiragino Sans GB", Arial, sans-serif;
      font-size: 12.5px;
      line-height: 1.7;
    }}
    h1 {{ font-size: 20px; margin: 0 0 8px 0; }}
    h2 {{ font-size: 15px; margin: 18px 0 8px; }}
    p {{ margin: 6px 0; }}
    .publish-date {{ color: #444; }}
    .agreement-content {{ max-width: 720px; }}
    b {{ font-weight: 700; }}
    /* Avoid ugly page breaks */
    h1, h2 {{ break-after: avoid-page; page-break-after: avoid; }}
    p {{ orphans: 3; widows: 3; }}
  </style>
</head>
<body>
{content}
</body>
</html>
"""


def extract_agreement_html(sql: str, agreement_type: str) -> str:
    # Extract the content argument (3rd value) of INSERT INTO agreements ... VALUES (...)
    # We specifically match the type and then capture the following quoted HTML content.
    # Content in our migrations uses a single-quoted string without embedded single-quotes.
    pattern = re.compile(
        r"INSERT\s+INTO\s+agreements\s*\([^)]*\)\s*\n?VALUES\s*\(\s*\n?\s*'"
        + re.escape(agreement_type)
        + r"'\s*,\s*\n?\s*'[^']*'\s*,\s*\n?\s*'(?P<html>[\s\S]*?)'\s*,\s*\n?\s*'v1\.2\.0-legal'",
        re.IGNORECASE,
    )
    m = pattern.search(sql)
    if not m:
        raise SystemExit(f"Cannot find agreement type {agreement_type} in {SQL_PATH}")
    return m.group("html")


def write_html_files(sql_text: str) -> Dict[str, Tuple[pathlib.Path, str]]:
    outputs: Dict[str, Tuple[pathlib.Path, str]] = {}
    for t, name in TARGETS.items():
        content = extract_agreement_html(sql_text, t)
        title = f"乐客来福{name}（v1.2.0-legal）"
        full_html = HTML_SHELL.format(title=title, content=content)
        html_path = OUT_DIR / f"{t}.html"
        html_path.write_text(full_html, encoding="utf-8")
        outputs[t] = (html_path, title)
    return outputs


def print_pdf(html_path: pathlib.Path, pdf_path: pathlib.Path) -> None:
    url = html_path.resolve().as_uri()
    cmd = [
        "chromium",
        "--headless",
        "--disable-gpu",
        "--no-sandbox",
        f"--print-to-pdf={pdf_path}",
        "--print-to-pdf-no-header",
        url,
    ]
    subprocess.run(cmd, check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)


def main() -> None:
    if not SQL_PATH.exists():
        raise SystemExit(f"SQL file not found: {SQL_PATH}")

    OUT_DIR.mkdir(parents=True, exist_ok=True)

    sql_text = SQL_PATH.read_text(encoding="utf-8")
    html_outputs = write_html_files(sql_text)

    for t, (html_path, _title) in html_outputs.items():
        pdf_path = OUT_DIR / f"{t}.pdf"
        print_pdf(html_path, pdf_path)

    print("OK")
    for t in TARGETS.keys():
        print(f"- {t}: {OUT_DIR / (t + '.pdf')}")


if __name__ == "__main__":
    main()
