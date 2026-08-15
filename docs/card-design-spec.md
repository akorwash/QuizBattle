# QuizBattle card design specification

> **Status:** implemented MVP design contract. The Arabic RTL shell, question/collection cards, match states, market, trades, and responsive rules described here are now present. Section 12 is retained as the historical before-state that motivated the redesign.

## 1. Design decision

The recommended direction is **"The Knowledge Duel Codex"**: a modern Arabic knowledge archive presented as a competitive dueling table. Cards should feel like engraved knowledge tablets rather than casino cards or a generic SaaS dashboard.

The visual signature is a thin diagonal **knowledge blade** running from the top-inline corner toward the center. On a live question it becomes the remaining-time rail; on a collectible card it becomes the power/history rail. This gives the product one memorable motif without filling every surface with decoration.

The intended audience is Arabic-first players on phones, with desktop support for longer sessions. The default document language and direction should therefore be `lang="ar" dir="rtl"`; English names and technical identifiers remain isolated with `dir="auto"` or `dir="ltr"`.

Design principles:

- Knowledge first: the question is always the strongest visual element.
- Competition through structure, not noise: clear timers, turn ownership, card power, and match status.
- Material, not glossy: ink, parchment, oxidized brass, and one restrained red signal.
- State is never communicated by color alone.
- No card animation, rarity shine, or marketplace affordance may imply a backend feature that does not yet exist.

## 2. Palette

Use these six named colors as the core palette. Opacity variants may be derived from them; do not introduce a second accent family without revisiting the system.

| Token | Hex | Role | Usage constraints |
| --- | --- | --- | --- |
| `ink-950` | `#0E2024` | Primary text, night background, card frame | Default text on light surfaces |
| `parchment-100` | `#F7EBD1` | Main card face, readable content surface | Pair with `ink-950`; never use white text on it |
| `sand-300` | `#D7C49A` | Dividers, disabled surface, secondary parchment | Decorative and inactive UI only |
| `brass-500` | `#C08A2B` | Power, selected outline, rare tier | Do not use as small text on parchment; pair with ink |
| `verdigris-700` | `#16665F` | Primary action, online/ready, notable tier | White or parchment text is allowed |
| `vermilion-700` | `#B33A31` | Deadline, destructive action, legendary detail | Always add an icon/text label; never signal error by red alone |

Primary combinations:

- App shell: `ink-950` background with `parchment-100` content.
- Card body: `parchment-100` with `ink-950` text and `sand-300` dividers.
- Primary action: `verdigris-700` with a light foreground.
- Selected card: `brass-500` outline plus a visible check badge and `aria-pressed="true"`.
- Danger/closing seconds: `vermilion-700` plus a textual status or icon.

Avoid dominant blue/purple gradients, glassmorphism, neon glows, and rainbow rarity borders. Texture should be generated with subtle CSS lines/noise at very low opacity, not a large background image.

## 3. Typography

Do not add a font CDN. Start with a robust Arabic system stack and self-host a font only after measuring a genuine need.

```css
--qb-font-body: system-ui, "Segoe UI", Tahoma, Arial, sans-serif;
--qb-font-display: Tahoma, "Segoe UI", Arial, sans-serif;
--qb-font-mono: ui-monospace, "Cascadia Mono", Consolas, monospace;
```

Roles:

| Role | Size / line-height | Weight | Notes |
| --- | --- | --- | --- |
| Display / match title | `clamp(1.5rem, 1.3rem + .8vw, 2rem) / 1.3` | 800 | One or two lines; no decorative letter spacing |
| Live question | `clamp(1.25rem, 1.12rem + .55vw, 1.625rem) / 1.65` | 700 | Never truncate the active question |
| Card title | `1rem / 1.45` | 750 | Two-line clamp is allowed only in a collection grid |
| Body / answer | `1rem / 1.6` | 500 | Keep at least 16px on form controls |
| Utility / metadata | `.8125rem / 1.45` | 650 | Category, rarity, counters; not all uppercase |
| Numeric score/timer | `1.125rem / 1` | 800, tabular numbers | Use `font-variant-numeric: tabular-nums` |

Render user-provided names using `dir="auto"`. Render battle/card IDs using the mono stack and `dir="ltr"`, but IDs should not dominate the card.

## 4. Card families and anatomy

### 4.1 Live question card

This is a responsive gameplay surface, not a fixed-height poster. Use `width: min(100%, 30rem)` and `min-height`, with content allowed to grow. A fixed aspect ratio must never clip Arabic text.

Anatomy, from top to bottom:

1. **Context bar:** round number, whose turn it is, and connection state.
2. **Category seal:** local icon/pattern plus the full category label, for example `جغرافيا`.
3. **Difficulty/rarity label:** explicit text and symbols; no answer metadata.
4. **Question:** full server-supplied question text. The correct answer must not exist in the DOM or browser payload before resolution.
5. **Answer group:** semantic `fieldset`/`legend`; two to four full-width answer buttons with stable positions.
6. **Commit status:** `لم يتم الإرسال`, `تم تثبيت الإجابة`, or a recoverable error.
7. **Knowledge blade / timer:** a progress rail with numeric remaining time. Use `role="progressbar"`; announce only meaningful thresholds, not every second.
8. **Footer:** card power and public history metrics only if the server has authorized them.

Answer states:

- `idle`: neutral outline and no preselected option.
- `selected`: brass inset outline, check icon, and `aria-checked="true"`.
- `submitted`: answer positions freeze; show `تم تثبيت الإجابة` and disable repeat submission.
- `resolved-correct`: verdigris border plus `إجابة صحيحة` and an icon.
- `resolved-wrong`: vermilion border plus `إجابة غير صحيحة`; also reveal the correct answer only after the server resolves the round.
- `expired`: timer reads `انتهى الوقت`; controls are disabled and focus moves to the round result heading, not an overlay.
- `reconnecting`: preserve the last confirmed server snapshot and show a non-modal status strip. Do not let the client guess the outcome.

### 4.2 Collectible knowledge card

Use a stable `5 / 7` ratio in collection grids: approximately 156x218px on small phones, 200x280px on tablets, and up to 224x314px on desktop. A detail view may grow vertically.

Anatomy:

1. **Engraved frame:** rarity material/pattern and a category notch.
2. **Header:** category label and rarity name/symbol count.
3. **Subject field:** a local category sigil or geometric pattern; avoid generic stock photographs.
4. **Card title:** short curator-written subject title, not the answer.
5. **Question preview:** up to three lines in grid view; full text in an authorized detail view.
6. **Power rail:** numeric power plus a short accessible label.
7. **History summary:** wins/defenses/transfers as server-derived counts.
8. **Ownership/state footer:** `مملوكة لك`, `في مباراة`, `معروضة للتبادل`, or `انتقلت الملكية`.
9. **Action zone:** select, inspect, offer, or cancel. Only one primary action per card.

The reverse face is metadata, not an answer reveal. If a flip is implemented, it must be activated by a labeled button and keyboard, never hover alone.

## 5. Category language

Each category needs three redundant cues: Arabic label, a locally shipped icon, and a restrained frame pattern. Color is optional and must not be the only cue.

Recommended first set:

| Category | Icon concept | Pattern |
| --- | --- | --- |
| علوم | atom/orbit | concentric dots |
| دين وثقافة إسلامية | geometric eight-point motif | fine interlace; avoid sacred text as decoration |
| رياضيات | compass/triangle | graph grid |
| مدن ومعالم | arch/gate | stepped skyline |
| جغرافيا | compass rose | contour lines |
| تاريخ | column/scroll | timeline ticks |
| سياسة ومواطنة | civic building | balanced vertical lines |
| لغة وأدب | open book/pen | manuscript rules |
| تقنية | circuit node | circuit traces |
| رياضة | laurel/ball | scoreboard marks |

Icons should be local SVG assets with consistent 20/24px geometry and useful alternative text when informative. Do not use emoji as the final category system because appearance differs by platform.

## 6. Rarity and card state

Rarity is intrinsic; interaction states are orthogonal. The DOM contract is `data-rarity` plus one or more state classes.

### Rarity tiers

| `data-rarity` | Arabic label | Visual treatment | Non-color cue |
| --- | --- | --- | --- |
| `common` | شائعة | Ink frame, sand divider | One diamond pip |
| `notable` | مميزة | Verdigris corner plate | Two diamond pips |
| `rare` | نادرة | Brass double frame | Three diamond pips |
| `legendary` | أسطورية | Ink/brass frame with one vermilion seal | Four pips plus crown/seal shape |

Do not animate common/rare cards continuously. A legendary card may have a single 450ms reveal sweep when first acquired, never an endless shimmer.

### Interaction/ownership states

| State | Class / ARIA | Required presentation and behavior |
| --- | --- | --- |
| Default | `.qb-card` | Resting elevation, full content available |
| Hover-capable preview | `.qb-card:hover` under `@media (hover:hover)` | Raise by 2px; no hidden action appears only on hover |
| Keyboard focus | `:focus-visible` | 3px high-contrast outline with 3px offset |
| Selected | `.is-selected`, `aria-pressed="true"` | Brass outline, check badge, text `محددة` |
| Locked / escrow | `.is-locked`, `aria-disabled="true"` | Sand veil, lock icon, reason (`في مباراة رقم …`), actions disabled |
| Trade pending | `.is-trade-pending` | Clock icon and `عرض تبادل معلّق`; cancel remains available when authorized |
| Traded / ownership changed | `.is-traded`, `data-owned="false"` | Transfer stamp, new-owner text if public, ownership actions removed |
| Unavailable | `.is-unavailable`, disabled control | Neutral disabled treatment plus a reason; not opacity alone |

State priority when combined:

1. Locked/escrow overrides selected actions.
2. Traded overrides ownership and selection.
3. Pending trade prevents a second offer.
4. Rarity frame remains visible but never obscures the state label.

Completed trade is an event in the ownership ledger, not merely a client-side class. The UI should render it only from a server-confirmed snapshot/event.

## 7. Text wireframes

### Mobile live round (320-430px)

```text
┌──────────────────────────────────┐
│ الجولة ٣/٥       دورك الآن   ● متصل │
├──────────────────────────────────┤
│ [جغرافيا]                  ◆◆ نادرة │
│                                  │
│ ما المدينة التي يمر بها نهر ...؟ │
│                                  │
│ ○ الإجابة الأولى                 │
│ ○ الإجابة الثانية                │
│ ○ الإجابة الثالثة                │
│ ○ الإجابة الرابعة                │
│                                  │
│ تم اختيار إجابة — لم تُثبت بعد   │
│━━━━━━━ الوقت المتبقي ١٢ث ━━━━━━━│
│ القوة ٧٢             ١٤ دفاعًا    │
└──────────────────────────────────┘
┌──── sticky safe-area action ─────┐
│          [تثبيت الإجابة]          │
└──────────────────────────────────┘
```

The action bar respects `env(safe-area-inset-bottom)`. The question scrolls with the page only if needed; answer buttons never sit underneath the sticky action.

### Desktop battle arena (>=1024px)

```text
┌──────── player/opponent ──────── score/round ─────── connection ─┐
│                                                                  │
│  participants/status     [ live question card ]      match log   │
│  and committed cards     [       max 30rem       ]   / chat dock │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

The center question card remains the only dominant object. Chat/log may collapse into a drawer; it must not squeeze the question below 320px.

### Collection grid

```text
┌ filter/sort/search ───────────────────────────── selected 2/5 ┐
│ ┌──── card ────┐  ┌──── card ────┐  ┌──── card ────┐         │
│ │ category ◆◆  │  │ locked 🔒    │  │ traded ↗    │         │
│ │ subject mark │  │ subject mark │  │ subject mark │         │
│ │ title        │  │ title        │  │ title        │         │
│ │ power/history│  │ reason       │  │ new owner    │         │
│ └──────────────┘  └──────────────┘  └──────────────┘         │
└───────────────────────────────────────────────────────────────┘
```

On phones, use two columns only when the viewport can keep each card at least 148px wide; otherwise use one compact horizontal card per row. Never use a swipe gesture as the sole route to an action.

## 8. Mobile-first layout contract

- Base viewport: 360x800 CSS pixels; support down to 320px without horizontal scrolling.
- App content width: `min(100% - 2rem, 75rem)`; question column max 30rem.
- Use CSS Grid/Flex and logical properties (`margin-inline`, `padding-block`, `inset-inline-*`). Remove directional floats.
- Touch targets: at least 44x44px with 8px separation for adjacent destructive/primary actions.
- Sticky bottom actions use `padding-bottom: calc(var(--qb-space-3) + env(safe-area-inset-bottom))`.
- At 768px, card grids can expand to three columns and lobby panes can sit side by side.
- At 1024px, the battle becomes a three-zone arena. Do not scale type directly with viewport width.
- Landscape phones use a two-column question/answer layout only when it leaves at least 44px answer targets and readable lines.
- Chat is a dedicated community/lobby surface on mobile, reachable from bottom navigation; it must not remain embedded only in the profile form.

## 9. Motion contract

Motion explains a state change:

- Press: 90ms scale to `.985`, only while actively pressed.
- Card selection: 160ms outline/badge transition.
- Drawer or detail: 220ms translate/fade.
- Server-confirmed acquisition: one 450ms blade sweep.
- Timer: linear width change, but accessible text updates at controlled thresholds.
- Reordering cards: 180ms position transition only after the server confirms order/state.

```css
@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after {
    scroll-behavior: auto !important;
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 0.01ms !important;
  }
}
```

The reduced-motion treatment must also cover the existing spinner. Loading feedback should retain visible text such as `جاري تحميل المباراة…` when animation is disabled.

## 10. Accessibility contract

- Set page language/direction accurately; use logical CSS properties throughout.
- Meet WCAG AA: at least 4.5:1 for normal text, 3:1 for large text and essential control boundaries.
- Preserve a visible `:focus-visible` ring; do not remove outlines.
- Cards used as selectors are buttons or contain a clearly labeled button. A plain `div` is never the interactive target.
- Answer choices use `fieldset`, `legend`, and native radio semantics or an equivalent fully keyboard-operable radiogroup.
- The timer has a visual rail and readable seconds. Screen-reader announcements occur at start, 10 seconds, 5 seconds, and expiry—not every tick.
- Live round results use a polite live region; urgent connection loss may use assertive status once.
- `aria-expanded` and `aria-controls` track drawers, chat panels, and the existing battle participant expansion.
- Category, rarity, selected, locked, and traded states always include text/symbol semantics independent of color.
- Do not truncate the active question, correct answer after resolution, lock reason, trade status, or errors.
- Use `overflow-wrap: anywhere` for long names and mixed-language content. Use `dir="auto"` for chat messages.
- Support `forced-colors: active`: do not rely on backgrounds or shadows to convey selection.
- Never move focus unexpectedly on WebSocket refresh. Preserve the focused card by stable card ID.

## 11. CSS token and component contract

Create one external design stylesheet (suggested: `src/static/quizbattle.css`) and remove page-level inline styles rather than growing `home.css` with more overrides.

```css
:root {
  --qb-ink: #0E2024;
  --qb-paper: #F7EBD1;
  --qb-sand: #D7C49A;
  --qb-brass: #C08A2B;
  --qb-verdigris: #16665F;
  --qb-vermilion: #B33A31;

  --qb-font-body: system-ui, "Segoe UI", Tahoma, Arial, sans-serif;
  --qb-font-display: Tahoma, "Segoe UI", Arial, sans-serif;
  --qb-font-mono: ui-monospace, "Cascadia Mono", Consolas, monospace;

  --qb-space-1: .25rem;
  --qb-space-2: .5rem;
  --qb-space-3: .75rem;
  --qb-space-4: 1rem;
  --qb-space-6: 1.5rem;
  --qb-space-8: 2rem;

  --qb-radius-sm: .5rem;
  --qb-radius-md: .875rem;
  --qb-radius-card: 1.125rem;
  --qb-border: 1px solid color-mix(in srgb, var(--qb-ink) 22%, transparent);
  --qb-focus: 0 0 0 3px var(--qb-paper), 0 0 0 6px var(--qb-brass);
  --qb-shadow-card: 0 .75rem 2rem rgb(14 32 36 / .22);
  --qb-motion-fast: 90ms;
  --qb-motion-state: 160ms;
  --qb-motion-panel: 220ms;
}
```

Required class/data contract:

```text
.qb-app-shell
.qb-topbar
.qb-bottom-nav
.qb-arena
.qb-card-grid
.qb-card
  .qb-card--question | .qb-card--collectible | .qb-card--compact
  [data-card-id]
  [data-category]
  [data-rarity="common|notable|rare|legendary"]
  .is-selected | .is-locked | .is-trade-pending | .is-traded
.qb-card__blade
.qb-card__header
.qb-card__category
.qb-card__rarity
.qb-card__subject
.qb-card__title
.qb-card__question
.qb-card__answers
.qb-answer
.qb-card__stats
.qb-card__state
.qb-card__actions
.qb-match-status
.qb-community-panel
.qb-empty-state
```

JavaScript should render state through these semantic attributes/classes and ARIA values. It must not write inline styles to represent timer, rarity, or selection; expose numeric progress through a CSS custom property only if the CSP permits a nonce-safe mechanism, otherwise use a native `<progress>` element.

## 12. Historical review of the pre-redesign interface

The findings below describe the interface before the 2026-08-15 implementation pass; they are not claims about the current tree.

### High-impact problems

1. Every view under `src/api/view/*.html` declares `lang="en"` and lacks `dir="rtl"`, although the desired product is Arabic-first. Navigation, forms, errors, and game labels are English-only.
2. There is no question-card or collectible-card component. `battle.html` contains an empty `#battle_container`, while `battle.js` intentionally disables Start because the authoritative game engine is absent. The UI must not pretend full play exists until that backend work lands.
3. `src/api/view/user.html` contains two inline `<style>` blocks, global `img` rules, left/right floats, and English directional classes. They conflict with the hardened CSP and make RTL/mobile behavior fragile.
4. `src/static/home.css` uses `height: 100%`, fixed/relative floating battle actions, global white links, global text shadow, and a 42em cover template. This can overlap controls, clip tall content, and produces poor contrast on light Bootstrap panels.
5. The world chat is embedded inside a three-column profile/account page rather than presented as a community lobby. On narrow screens it becomes a long sequence of unrelated panels, and fixed `height: 350px` does not account for the mobile keyboard.

### Medium-impact problems

1. `home.css` and `auth.css` duplicate old cover-template rules and physical `margin-left`/float direction. Consolidate into a logical-property design system.
2. `loading.css` runs an infinite 10em spinner with no reduced-motion treatment or visible loading text. `battle.html` does not apply the `.loader` class to `#battlepage`, so the intended loader is not reliably visible.
3. `game.js`, `battle.js`, and `worldchat.js` generate generic Bootstrap rows with English copy. There are no semantic empty, reconnecting, capacity, selected, locked, or ownership states.
4. `Create Game` uses the destructive red button style, while `Start` uses an informational cyan style. Action hierarchy is inconsistent with meaning.
5. The current expand control has no `aria-expanded`/`aria-controls`, and toggling layout classes does not update its accessible name.
6. The current glossy green/blue `src/static/icon.png` belongs to a different visual language than the proposed ink/parchment/brass cards. Keep it temporarily as favicon/avatar only; replace it later with a simplified local mark after the card system is proven.

### Lower-impact polish gaps

- Battle lists lack designed loading, empty, full, and closed states.
- Player capacity is shown as a raw joined count rather than `3/10` with availability.
- Errors live in small inline spans and can be separated from the action that failed.
- The layout has no mobile bottom navigation, safe-area treatment, or persistent match status.
- Generic icon/avatar reuse gives every chat participant the same visual identity.

## 13. Implemented file map

The implementation landed across the following files/elements:

| File | Required implementation |
| --- | --- |
| `src/static/quizbattle.css` (new) | Tokens, app shell, card families, states, RTL, breakpoints, focus, forced-colors, reduced-motion |
| `src/api/view/*.html` | Set Arabic language/direction, adopt shared shell/navigation, remove physical/directional markup |
| `src/api/view/battle.html` | Add semantic arena, live question host, match status, participants drawer, accessible loading/empty states |
| `src/api/view/gameplay.html` | Replace generic white panel with lobby cards, capacity/status, filters, and designed empty/loading states |
| `src/api/view/user.html` | Remove inline styles; split profile, community chat, and future collection/trade surfaces into clear routes or tabs |
| `src/static/game.js` | Render `.qb-lobby-*` components with explicit loading/empty/full/closed states and Arabic copy |
| `src/static/battle.js` | Render card states only from future authoritative snapshots; manage focus, `aria-expanded`, timer/result semantics |
| `src/static/worldchat.js` | Render logical RTL message rows, `dir="auto"`, delivery/connection status, and a mobile keyboard-safe composer |
| `src/static/loading.css` | Retire into the shared stylesheet or add reduced-motion and textual loading behavior |
| `src/static/icon.png` / future local assets | Keep current icon temporarily; introduce a coherent local logo and category SVG set without a new CDN |

Marketplace actions, balances, card selection, power/mastery displays, and answer resolution are enabled only because their server-authoritative APIs and transactional ledger now exist. Any future client feature must follow the same rule.

## 14. Acceptance checklist

The visual implementation is ready for approval only when all of the following pass:

- 320px, 360px, 390px, 768px, 1024px, and 1440px layouts have no horizontal scroll, overlap, or clipped active questions.
- Arabic RTL and mixed Arabic/English player names render correctly.
- Keyboard-only users can select a card, choose/submit an answer, open drawers, and reach chat without a trap.
- Screen readers receive category, rarity, selected/locked/traded state, timer thresholds, and round results.
- Default, selected, locked, pending trade, traded, correct, wrong, expired, reconnecting, full lobby, and empty states have screenshot coverage.
- Light/card and dark/shell contrast meet WCAG AA; focus is visible in normal and forced-colors modes.
- `prefers-reduced-motion` removes spinner/flip/sweep movement while retaining understandable feedback.
- A 1,000-character Arabic question and long user names wrap safely; the active question is never line-clamped.
- No correct answer is present in pre-resolution HTML, JSON, DOM attributes, logs, or client state.
- Touch controls stay at least 44x44px and remain above mobile safe areas and the virtual keyboard.
- Visual states are driven by confirmed server state, not optimistic ownership or score changes.
