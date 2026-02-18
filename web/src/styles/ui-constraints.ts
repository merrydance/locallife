export const UI_COMPONENT_WHITELIST = [
  "Card",
  "PageShell",
  "PageHeader",
  "PageContent",
  "Tabs",
  "Table",
  "Select",
  "Input",
  "Textarea",
  "Button",
  "Badge",
  "Dialog",
  "AlertDialog",
  "Label",
  "Switch",
] as const;

export const UI_CLASS_CONSTRAINTS = {
  layout: [
    "space-y-4",
    "space-y-6",
    "grid gap-4 md:grid-cols-2 xl:grid-cols-4",
    "flex flex-col gap-3 md:flex-row md:items-center",
  ],
  surface: [
    "rounded-lg border bg-background",
    "bg-muted/40 border",
  ],
  typography: [
    "text-sm",
    "text-muted-foreground",
    "text-2xl font-semibold",
  ],
  feedback: [
    "border-destructive/30 bg-destructive/5 text-destructive",
  ],
  tabs: [
    "Use shared style in components/ui/tabs.tsx",
    "Active state must be visually dominant",
  ],
} as const;

export const UI_COPY_BANNED_TERMS = [
  "与小程序一致",
  "debug",
  "proxy",
  "fallback",
] as const;

export type UiConstraintGroup = keyof typeof UI_CLASS_CONSTRAINTS;
