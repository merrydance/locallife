"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Check, Loader2, Search } from "lucide-react";
import { toast } from "sonner";
import { apiGet } from "@/lib/api";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { ScrollArea } from "@/components/ui/scroll-area";
import type {
  ApplymentAccountType,
  ApplymentBankListResponse,
  ApplymentBankOption,
  ApplymentBankSearchResponse,
  ApplymentBindBankPayload,
  ApplymentBranchListResponse,
  ApplymentBranchOption,
  ApplymentCityListResponse,
  ApplymentCityOption,
  ApplymentProvinceListResponse,
  ApplymentProvinceOption,
} from "@/types/applyment-bank";

interface ApplymentBankFormProps {
  apiBasePath: string;
  defaultAccountType: ApplymentAccountType;
  submitting: boolean;
  submitLabel: string;
  onSubmit: (payload: ApplymentBindBankPayload) => Promise<void>;
  onCancel: () => void;
}

type BindBankFormState = ApplymentBindBankPayload;

const emptyBankFormState = (
  accountType: ApplymentAccountType,
): BindBankFormState => ({
  account_type: accountType,
  account_bank: "",
  account_bank_code: undefined,
  bank_alias: "",
  bank_alias_code: "",
  need_bank_branch: false,
  bank_address_code: "",
  bank_branch_id: "",
  bank_name: "",
  account_number: "",
  account_name: "",
  contact_phone: "",
  contact_email: "",
});

function normalizeKeyword(value: string) {
  return value.trim().toLowerCase();
}

function bankMatchesKeyword(bank: ApplymentBankOption, keyword: string) {
  if (!keyword) return true;
  const normalized = normalizeKeyword(keyword);
  return [
    bank.bank_alias,
    bank.account_bank,
    String(bank.account_bank_code ?? ""),
  ].some((item) => item.toLowerCase().includes(normalized));
}

function branchMatchesKeyword(branch: ApplymentBranchOption, keyword: string) {
  if (!keyword) return true;
  const normalized = normalizeKeyword(keyword);
  return [branch.bank_branch_name, branch.bank_branch_id].some((item) =>
    item.toLowerCase().includes(normalized),
  );
}

function bankDisplayLabel(bank: ApplymentBankOption) {
  if (bank.account_bank === bank.bank_alias) {
    return bank.bank_alias;
  }
  return `${bank.bank_alias} · 进件开户银行填写为${bank.account_bank}`;
}

export function ApplymentBankForm({
  apiBasePath,
  defaultAccountType,
  submitting,
  submitLabel,
  onSubmit,
  onCancel,
}: ApplymentBankFormProps) {
  const [form, setForm] = useState<BindBankFormState>(() =>
    emptyBankFormState(defaultAccountType),
  );
  const [banksByType, setBanksByType] = useState<
    Partial<Record<ApplymentAccountType, ApplymentBankOption[]>>
  >({});
  const [loadingBanks, setLoadingBanks] = useState(false);
  const [recognizingBank, setRecognizingBank] = useState(false);
  const [recognizedBanks, setRecognizedBanks] = useState<ApplymentBankOption[]>(
    [],
  );
  const [recognitionHint, setRecognitionHint] = useState("");
  const [bankKeyword, setBankKeyword] = useState("");
  const [branchKeyword, setBranchKeyword] = useState("");
  const [provinces, setProvinces] = useState<ApplymentProvinceOption[]>([]);
  const [cities, setCities] = useState<ApplymentCityOption[]>([]);
  const [branches, setBranches] = useState<ApplymentBranchOption[]>([]);
  const [loadingProvinces, setLoadingProvinces] = useState(false);
  const [loadingCities, setLoadingCities] = useState(false);
  const [loadingBranches, setLoadingBranches] = useState(false);
  const [selectedProvinceCode, setSelectedProvinceCode] = useState("");
  const [selectedCityCode, setSelectedCityCode] = useState("");

  const banks = useMemo(
    () => banksByType[form.account_type] ?? [],
    [banksByType, form.account_type],
  );

  const filteredBanks = useMemo(
    () => banks.filter((bank) => bankMatchesKeyword(bank, bankKeyword)),
    [banks, bankKeyword],
  );

  const filteredBranches = useMemo(
    () => branches.filter((branch) => branchMatchesKeyword(branch, branchKeyword)),
    [branches, branchKeyword],
  );

  const selectedProvince = provinces.find(
    (item) => String(item.province_code) === selectedProvinceCode,
  );
  const selectedCity = cities.find(
    (item) => String(item.city_code) === selectedCityCode,
  );

  const loadBanks = useCallback(
    async (accountType: ApplymentAccountType) => {
      if (banksByType[accountType]) {
        return;
      }
      setLoadingBanks(true);
      try {
        const response = await apiGet<ApplymentBankListResponse>(
          `${apiBasePath}/banks`,
          { account_type: accountType },
        );
        setBanksByType((prev) => ({ ...prev, [accountType]: response.banks }));
      } catch (error: unknown) {
        toast.error(error instanceof Error ? error.message : "加载银行列表失败");
      } finally {
        setLoadingBanks(false);
      }
    },
    [apiBasePath, banksByType],
  );

  const loadProvinces = useCallback(async () => {
    if (provinces.length > 0) {
      return;
    }
    setLoadingProvinces(true);
    try {
      const response = await apiGet<ApplymentProvinceListResponse>(
        `${apiBasePath}/areas/provinces`,
      );
      setProvinces(response.provinces);
    } catch (error: unknown) {
      toast.error(error instanceof Error ? error.message : "加载省份列表失败");
    } finally {
      setLoadingProvinces(false);
    }
  }, [apiBasePath, provinces.length]);

  const loadCities = async (provinceCode: string) => {
    if (!provinceCode) {
      return;
    }
    setLoadingCities(true);
    try {
      const response = await apiGet<ApplymentCityListResponse>(
        `${apiBasePath}/areas/provinces/${provinceCode}/cities`,
      );
      setCities(response.cities);
    } catch (error: unknown) {
      toast.error(error instanceof Error ? error.message : "加载城市列表失败");
    } finally {
      setLoadingCities(false);
    }
  };

  const loadBranches = async (bankAliasCode: string, cityCode: string) => {
    if (!bankAliasCode || !cityCode) {
      return;
    }
    setLoadingBranches(true);
    try {
      const response = await apiGet<ApplymentBranchListResponse>(
        `${apiBasePath}/banks/${bankAliasCode}/branches`,
        { city_code: cityCode },
      );
      setBranches(response.branches);
      setForm((prev) => ({
        ...prev,
        account_bank: response.account_bank || prev.account_bank,
        account_bank_code: response.account_bank_code || prev.account_bank_code,
        bank_alias: response.bank_alias || prev.bank_alias,
        bank_alias_code: response.bank_alias_code || prev.bank_alias_code,
      }));
    } catch (error: unknown) {
      toast.error(error instanceof Error ? error.message : "加载支行列表失败");
    } finally {
      setLoadingBranches(false);
    }
  };

  useEffect(() => {
    void loadBanks(form.account_type);
  }, [form.account_type, loadBanks]);

  useEffect(() => {
    if (form.need_bank_branch) {
      void loadProvinces();
    }
  }, [form.need_bank_branch, loadProvinces]);

  const resetBankSelection = (accountType: ApplymentAccountType) => {
    setForm((prev) => ({
      ...prev,
      account_type: accountType,
      account_bank: "",
      account_bank_code: undefined,
      bank_alias: "",
      bank_alias_code: "",
      need_bank_branch: false,
      bank_address_code: "",
      bank_branch_id: "",
      bank_name: "",
    }));
    setRecognizedBanks([]);
    setRecognitionHint("");
    setBankKeyword("");
    setBranchKeyword("");
    setSelectedProvinceCode("");
    setSelectedCityCode("");
    setCities([]);
    setBranches([]);
  };

  const applySelectedBank = (bank: ApplymentBankOption) => {
    setForm((prev) => ({
      ...prev,
      account_bank: bank.account_bank,
      account_bank_code: bank.account_bank_code,
      bank_alias: bank.bank_alias,
      bank_alias_code: bank.bank_alias_code,
      need_bank_branch: bank.need_bank_branch,
      bank_address_code: bank.need_bank_branch ? prev.bank_address_code : "",
      bank_branch_id: bank.need_bank_branch ? prev.bank_branch_id : "",
      bank_name: bank.need_bank_branch ? prev.bank_name : "",
    }));
    setRecognitionHint(
      bank.need_bank_branch
        ? "该银行需要继续选择开户地区和支行。"
        : "已完成开户银行选择。",
    );
    if (!bank.need_bank_branch) {
      setSelectedProvinceCode("");
      setSelectedCityCode("");
      setCities([]);
      setBranches([]);
      setBranchKeyword("");
    }
  };

  const handleRecognizeBank = async () => {
    const accountNumber = form.account_number.trim();
    if (form.account_type !== "ACCOUNT_TYPE_PRIVATE" || accountNumber.length < 8) {
      return;
    }
    setRecognizingBank(true);
    setRecognizedBanks([]);
    setRecognitionHint("");
    try {
      const response = await apiGet<ApplymentBankSearchResponse>(
        `${apiBasePath}/banks/search-by-bank-account`,
        { account_number: accountNumber },
      );
      setRecognizedBanks(response.matches);
      if (response.matches.length === 1) {
        applySelectedBank(response.matches[0]);
        setRecognitionHint("已自动识别开户银行，可继续核对或直接提交。",
        );
      } else if (response.matches.length > 1) {
        setRecognitionHint(`识别到 ${response.matches.length} 家候选银行，请确认具体一家。`);
      } else {
        setRecognitionHint("暂时无法识别这张卡的开户银行，请在下方手动搜索选择。");
      }
    } catch (error: unknown) {
      toast.error(error instanceof Error ? error.message : "识别开户银行失败");
    } finally {
      setRecognizingBank(false);
    }
  };

  const canSubmit =
    form.account_bank.trim().length > 0 &&
    form.account_number.trim().length > 0 &&
    form.account_name.trim().length > 0 &&
    form.contact_phone.trim().length > 0 &&
    (!form.need_bank_branch ||
      (form.bank_address_code?.trim() &&
        form.bank_branch_id?.trim() &&
        form.bank_name?.trim()));

  const submit = async () => {
    if (!canSubmit || submitting) {
      return;
    }
    await onSubmit({
      ...form,
      contact_email: form.contact_email?.trim() || undefined,
      bank_alias: form.bank_alias?.trim() || undefined,
      bank_alias_code: form.bank_alias_code?.trim() || undefined,
      bank_address_code: form.bank_address_code?.trim() || undefined,
      bank_branch_id: form.bank_branch_id?.trim() || undefined,
      bank_name: form.bank_name?.trim() || undefined,
      need_bank_branch: form.need_bank_branch || undefined,
      account_bank_code: form.account_bank_code || undefined,
    });
  };

  return (
    <div className="space-y-5 rounded-lg border bg-muted/30 p-5">
      <div className="space-y-1">
        <div className="text-sm font-medium">填写结算账户</div>
        <p className="text-xs text-muted-foreground">
          系统会优先帮你识别开户银行，只有微信要求时才需要继续选择支行。
        </p>
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <div className="space-y-2">
          <Label>账户类型</Label>
          <Select
            value={form.account_type}
            onValueChange={(value: ApplymentAccountType) => resetBankSelection(value)}
          >
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="ACCOUNT_TYPE_BUSINESS">对公账户</SelectItem>
              <SelectItem value="ACCOUNT_TYPE_PRIVATE">对私账户</SelectItem>
            </SelectContent>
          </Select>
        </div>

        <div className="space-y-2">
          <Label htmlFor="applyment-account-name">
            开户名称 <span className="text-destructive">*</span>
          </Label>
          <Input
            id="applyment-account-name"
            value={form.account_name}
            onChange={(event) =>
              setForm((prev) => ({ ...prev, account_name: event.target.value }))
            }
            placeholder={
              form.account_type === "ACCOUNT_TYPE_BUSINESS"
                ? "请输入企业或机构全称"
                : "请输入持卡人姓名"
            }
          />
        </div>

        <div className="space-y-2 md:col-span-2">
          <Label htmlFor="applyment-account-number">
            银行账号 <span className="text-destructive">*</span>
          </Label>
          <div className="flex flex-col gap-2 md:flex-row">
            <Input
              id="applyment-account-number"
              value={form.account_number}
              onChange={(event) =>
                setForm((prev) => ({ ...prev, account_number: event.target.value }))
              }
              placeholder={
                form.account_type === "ACCOUNT_TYPE_PRIVATE"
                  ? "输入个人储蓄卡号后可自动识别开户银行"
                  : "请输入对公银行账户号码"
              }
            />
            {form.account_type === "ACCOUNT_TYPE_PRIVATE" && (
              <Button
                type="button"
                variant="outline"
                onClick={handleRecognizeBank}
                disabled={recognizingBank || form.account_number.trim().length < 8}
              >
                {recognizingBank ? (
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                ) : (
                  <Search className="mr-2 h-4 w-4" />
                )}
                识别开户行
              </Button>
            )}
          </div>
        </div>

        <div className="space-y-2">
          <Label htmlFor="applyment-contact-phone">
            联系手机 <span className="text-destructive">*</span>
          </Label>
          <Input
            id="applyment-contact-phone"
            type="tel"
            value={form.contact_phone}
            onChange={(event) =>
              setForm((prev) => ({ ...prev, contact_phone: event.target.value }))
            }
            placeholder="请输入用于微信审核联系的手机号"
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="applyment-contact-email">联系邮箱</Label>
          <Input
            id="applyment-contact-email"
            type="email"
            value={form.contact_email}
            onChange={(event) =>
              setForm((prev) => ({ ...prev, contact_email: event.target.value }))
            }
            placeholder="可选，用于补充联系"
          />
        </div>
      </div>

      <div className="space-y-3">
        <div className="flex items-center justify-between gap-2">
          <div>
            <div className="text-sm font-medium">
              开户银行 <span className="text-destructive">*</span>
            </div>
            <p className="text-xs text-muted-foreground">
              请选择微信支持的开户银行。对私账户可先识别，再校对确认。
            </p>
          </div>
          {loadingBanks && <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />}
        </div>

        {recognitionHint && (
          <div className="rounded-md border border-sky-200 bg-sky-50 px-3 py-2 text-sm text-sky-700">
            {recognitionHint}
          </div>
        )}

        {form.bank_alias && (
          <div className="flex flex-wrap items-center gap-2 rounded-md border bg-background px-3 py-2 text-sm">
            <Badge variant="secondary">已选银行</Badge>
            <span>{form.bank_alias}</span>
            {form.account_bank !== form.bank_alias && (
              <span className="text-muted-foreground">
                微信开户银行将填写为 {form.account_bank}
              </span>
            )}
            {form.need_bank_branch && <Badge variant="outline">需选支行</Badge>}
          </div>
        )}

        {recognizedBanks.length > 1 && (
          <div className="space-y-2">
            <div className="text-xs font-medium text-muted-foreground">识别候选</div>
            <div className="flex flex-wrap gap-2">
              {recognizedBanks.map((bank) => (
                <Button
                  key={`${bank.bank_alias_code}-${bank.account_bank_code}`}
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => applySelectedBank(bank)}
                >
                  {bank.bank_alias}
                </Button>
              ))}
            </div>
          </div>
        )}

        <div className="space-y-2">
          <Input
            value={bankKeyword}
            onChange={(event) => setBankKeyword(event.target.value)}
            placeholder="搜索银行名称，例如 招商、工商、微众"
          />
          <ScrollArea className="h-56 rounded-md border bg-background">
            <div className="space-y-1 p-2">
              {filteredBanks.slice(0, 80).map((bank) => {
                const active =
                  form.bank_alias_code === bank.bank_alias_code &&
                  form.account_bank_code === bank.account_bank_code;
                return (
                  <button
                    key={`${bank.bank_alias_code}-${bank.account_bank_code}`}
                    type="button"
                    className="flex w-full items-start justify-between gap-3 rounded-md px-3 py-2 text-left text-sm hover:bg-muted"
                    onClick={() => applySelectedBank(bank)}
                  >
                    <div className="space-y-1">
                      <div className="font-medium">{bankDisplayLabel(bank)}</div>
                      <div className="text-xs text-muted-foreground">
                        开户银行编码 {bank.account_bank_code}
                      </div>
                    </div>
                    <div className="flex items-center gap-2">
                      {bank.need_bank_branch && (
                        <Badge variant="outline">需选支行</Badge>
                      )}
                      {active && <Check className="h-4 w-4 text-emerald-600" />}
                    </div>
                  </button>
                );
              })}
              {!loadingBanks && filteredBanks.length === 0 && (
                <div className="px-3 py-6 text-center text-sm text-muted-foreground">
                  没有匹配到银行，请换个关键词试试。
                </div>
              )}
            </div>
          </ScrollArea>
        </div>
      </div>

      {form.need_bank_branch && (
        <div className="space-y-4 rounded-md border bg-background p-4">
          <div className="space-y-1">
            <div className="text-sm font-medium">开户支行</div>
            <p className="text-xs text-muted-foreground">
              这家银行需要选择开户地址和支行联行号，系统会自动带入提交字段。
            </p>
          </div>

          <div className="grid gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <Label>开户省份</Label>
              <Select
                value={selectedProvinceCode}
                onValueChange={(value) => {
                  setSelectedProvinceCode(value);
                  setSelectedCityCode("");
                  setCities([]);
                  setBranches([]);
                  setBranchKeyword("");
                  setForm((prev) => ({
                    ...prev,
                    bank_address_code: "",
                    bank_branch_id: "",
                    bank_name: "",
                  }));
                  void loadCities(value);
                }}
              >
                <SelectTrigger>
                  <SelectValue placeholder={loadingProvinces ? "加载中..." : "请选择省份"} />
                </SelectTrigger>
                <SelectContent>
                  {provinces.map((province) => (
                    <SelectItem
                      key={province.province_code}
                      value={String(province.province_code)}
                    >
                      {province.province_name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <Label>开户城市</Label>
              <Select
                value={selectedCityCode}
                onValueChange={(value) => {
                  setSelectedCityCode(value);
                  setBranches([]);
                  setBranchKeyword("");
                  setForm((prev) => ({
                    ...prev,
                    bank_address_code: value,
                    bank_branch_id: "",
                    bank_name: "",
                  }));
                  if (form.bank_alias_code) {
                    void loadBranches(form.bank_alias_code, value);
                  }
                }}
                disabled={!selectedProvinceCode}
              >
                <SelectTrigger>
                  <SelectValue placeholder={loadingCities ? "加载中..." : "请选择城市"} />
                </SelectTrigger>
                <SelectContent>
                  {cities.map((city) => (
                    <SelectItem key={city.city_code} value={String(city.city_code)}>
                      {city.city_name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>

          <div className="space-y-2">
            <Label>支行名称</Label>
            <Input
              value={branchKeyword}
              onChange={(event) => setBranchKeyword(event.target.value)}
              disabled={!selectedCityCode}
              placeholder={
                selectedCityCode
                  ? "搜索支行名称或联行号"
                  : "请先选择开户城市"
              }
            />
            <ScrollArea className="h-48 rounded-md border bg-background">
              <div className="space-y-1 p-2">
                {loadingBranches && (
                  <div className="flex items-center gap-2 px-3 py-4 text-sm text-muted-foreground">
                    <Loader2 className="h-4 w-4 animate-spin" />
                    正在加载支行列表...
                  </div>
                )}
                {!loadingBranches &&
                  filteredBranches.slice(0, 80).map((branch) => {
                    const active = form.bank_branch_id === branch.bank_branch_id;
                    return (
                      <button
                        key={branch.bank_branch_id}
                        type="button"
                        className="flex w-full items-start justify-between gap-3 rounded-md px-3 py-2 text-left text-sm hover:bg-muted"
                        onClick={() =>
                          setForm((prev) => ({
                            ...prev,
                            bank_address_code: selectedCityCode,
                            bank_branch_id: branch.bank_branch_id,
                            bank_name: branch.bank_branch_name,
                          }))
                        }
                      >
                        <div className="space-y-1">
                          <div className="font-medium">{branch.bank_branch_name}</div>
                          <div className="text-xs text-muted-foreground">
                            联行号 {branch.bank_branch_id}
                          </div>
                        </div>
                        {active && <Check className="h-4 w-4 text-emerald-600" />}
                      </button>
                    );
                  })}
                {!loadingBranches && selectedCityCode && filteredBranches.length === 0 && (
                  <div className="px-3 py-6 text-center text-sm text-muted-foreground">
                    当前城市下没有匹配到支行，请尝试其他关键词。
                  </div>
                )}
              </div>
            </ScrollArea>
          </div>

          {(selectedProvince || selectedCity || form.bank_name) && (
            <div className="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-700">
              {selectedProvince && <span>{selectedProvince.province_name}</span>}
              {selectedProvince && selectedCity && <span> / </span>}
              {selectedCity && <span>{selectedCity.city_name}</span>}
              {form.bank_name && (
                <span>
                  {selectedCity ? " / " : ""}
                  {form.bank_name}
                </span>
              )}
            </div>
          )}
        </div>
      )}

      <div className="flex gap-2 pt-1">
        <Button type="button" onClick={submit} disabled={!canSubmit || submitting}>
          {submitting ? "提交中..." : submitLabel}
        </Button>
        <Button type="button" variant="outline" onClick={onCancel} disabled={submitting}>
          取消
        </Button>
      </div>
    </div>
  );
}