import { defineConfig, globalIgnores } from "eslint/config";
import nextVitals from "eslint-config-next/core-web-vitals";
import nextTs from "eslint-config-next/typescript";

const localUiRulesPlugin = {
  rules: {
    "prefer-select-for-fixed-options": {
      meta: {
        type: "suggestion",
        docs: {
          description: "Prefer Select over Input for fixed option fields.",
        },
        schema: [],
      },
      create(context) {
        const fixedOptionHint =
          /(状态|星期|周几|时段|时间段|等级|类型|pending|approved|rejected|suspended|deactivated|resolved|low|medium|high|critical)/i;
        const fixedOptionStateName = /(status|level|type|week|weekday|day|timeSlot)/i;

        function getLiteralAttr(node, attrName) {
          const attr = node.attributes.find(
            (attribute) => attribute.type === "JSXAttribute" && attribute.name?.name === attrName
          );
          if (!attr || !attr.value) return "";
          if (attr.value.type === "Literal" && typeof attr.value.value === "string") {
            return attr.value.value;
          }
          return "";
        }

        function getExpressionIdentifier(node, attrName) {
          const attr = node.attributes.find(
            (attribute) => attribute.type === "JSXAttribute" && attribute.name?.name === attrName
          );
          if (!attr || !attr.value || attr.value.type !== "JSXExpressionContainer") return "";
          const expr = attr.value.expression;
          if (expr.type === "Identifier") return expr.name;
          return "";
        }

        return {
          JSXOpeningElement(node) {
            if (node.name.type !== "JSXIdentifier" || node.name.name !== "Input") return;

            const placeholder = getLiteralAttr(node, "placeholder");
            const id = getLiteralAttr(node, "id");
            const valueIdentifier = getExpressionIdentifier(node, "value");

            const hasFixedOptionLiteralHint =
              fixedOptionHint.test(placeholder) || fixedOptionHint.test(id);
            const hasFixedOptionValueHint = fixedOptionStateName.test(valueIdentifier);

            if (!hasFixedOptionLiteralHint && !hasFixedOptionValueHint) return;

            context.report({
              node,
              message:
                "固定集合字段请优先使用 Select，而不是 Input（例如状态、等级、周几、时段、枚举值）。",
            });
          },
        };
      },
    },
    "no-raw-enum-display": {
      meta: {
        type: "problem",
        docs: {
          description: "Disallow raw English enum display in operator-facing UI.",
        },
        schema: [],
      },
      create(context) {
        const rawEnumPattern = /\b(pending|approved|rejected|suspended|deactivated|resolved|low|medium|high|critical)\b/i;

        function getJsxName(nodeName) {
          if (!nodeName) return "";
          if (nodeName.type === "JSXIdentifier") return nodeName.name;
          return "";
        }

        function getMemberPropName(expression) {
          if (!expression || expression.type !== "MemberExpression") return "";
          if (!expression.property) return "";
          if (expression.property.type === "Identifier") return expression.property.name;
          return "";
        }

        return {
          JSXElement(node) {
            const name = getJsxName(node.openingElement?.name);
            if (name !== "SelectItem") return;

            for (const child of node.children || []) {
              if (child.type === "JSXText" && rawEnumPattern.test(child.value.trim())) {
                context.report({
                  node: child,
                  message:
                    "禁止在业务界面直接显示英文枚举值，请改为中文业务文案或映射后的标签。",
                });
              }
            }
          },
          JSXExpressionContainer(node) {
            if (node.parent?.type === "JSXAttribute") return;

            const expr = node.expression;
            if (!expr) return;

            if (expr.type === "Identifier" && /^(status|level)$/i.test(expr.name)) {
              context.report({
                node,
                message:
                  "禁止直接渲染 status/level 原始枚举值，请使用映射函数（如 formatXxxStatus/formatXxxLevel）。",
              });
              return;
            }

            if (expr.type === "MemberExpression") {
              const propName = getMemberPropName(expr);
              if (/^(status|level)$/i.test(propName)) {
                context.report({
                  node,
                  message:
                    "禁止直接渲染对象的 status/level 字段，请使用映射函数输出中文文案。",
                });
              }
            }
          },
        };
      },
    },
  },
};

const eslintConfig = defineConfig([
  ...nextVitals,
  ...nextTs,
  {
    files: ["src/app/**/*.tsx", "src/components/**/*.tsx"],
    plugins: {
      localui: localUiRulesPlugin,
    },
    rules: {
      "localui/prefer-select-for-fixed-options": "error",
    },
  },
  {
    files: ["src/app/operator/**/*.tsx"],
    plugins: {
      localui: localUiRulesPlugin,
    },
    rules: {
      "localui/no-raw-enum-display": "error",
    },
  },
  {
    files: ["src/app/merchant/**/*.tsx", "src/app/platform/**/*.tsx"],
    plugins: {
      localui: localUiRulesPlugin,
    },
    rules: {
      "localui/no-raw-enum-display": "error",
    },
  },
  // Override default ignores of eslint-config-next.
  globalIgnores([
    // Default ignores of eslint-config-next:
    ".next/**",
    "out/**",
    "build/**",
    "next-env.d.ts",
  ]),
]);

export default eslintConfig;
