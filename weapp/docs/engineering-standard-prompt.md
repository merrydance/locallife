# LocalLife Engineering Standard & System Prompt (v1.0)

This document serves as the "Source of Truth" for AI coding assistants. Adhere to these standards strictly when modifying or expanding the codebase.

---

## 1. Responsive Layout Standard (v3.0)
We follow a **4-State** responsive model.

| State | Threshold (px) | Strategy |
| :--- | :--- | :--- |
| **Mobile** | < 750 | Mobile-first, single column. |
| **Tablet** | 750 ~ 1279 | Grid layouts (2-3 columns). |
| **PC-Window** | 1280 ~ 1599 | Dual-pane/Dashboard, sidebar collapsed. |
| **PC-Full** | >= 1600 | **Max-width constraint (1440px)**, multi-pane. |

### 🛑 Engineering Preference: Scheme A (match-media)
- **Do NOT** rely solely on CSS for complex layout shifts.
- **DO** use `<match-media>` in WXML to physically separate structures.
- Use `templates/` to isolate different viewport contents for maintainability.

---

## 2. Content Steering Policy
- **Management Roles (Merchant/Operator/Admin)**:
    - **Optimization Goal**: Efficiency & Functionality.
    - **Strategy**: Full functionality provided on PC/Tablet. Mobile version can be a "Simplified Dashboard" or a prompt to use PC.
- **Customer Side**:
    - **Optimization Goal**: Visual Perfection.
    - **Strategy**: Full responsiveness. On **PC-Full**, must use `.customer-page-container` to lock content width (1024px) to prevent unsightly stretching.

---

## 3. Styling & Infrastructure
- **CSS Variables**: Always use TDesign and LocalLife global variables (e.g., `--brand-coral`, `--font-size-base`).
- **Responsive Utilities**:
    - [utils/responsive.ts](file:///c:/ll/miniapp/miniprogram/utils/responsive.ts): Use `responsiveBehavior` for every page.
    - [styles/responsive.wxss](file:///c:/ll/miniapp/miniprogram/styles/responsive.wxss): Import on every page inheriting responsive classes.
- **Component Usage**: Prefer `tdesign-miniprogram` components over native ones.

---

## 4. Key Artifacts Location
- **Report & Plan**: `C:\Users\huarun\.gemini\antigravity\brain\34973f21-5943-4528-8e7d-782cae5ddd46`
- **Audit Report**: `audit_report.md`
- **Responsive Standard**: `responsive_design_standard.md`
- **Cleanup Report**: `walkthrough.md`

---
**Instruction for AI**: Before any code modification, read this prompt and the `responsive_design_standard.md`. Ensure every new page follows the match-media template pattern.
