---
applyTo: "**/*.arb,**/*.po"
excludeAgent: "coding-agent"
---

# Translation review instructions

When reviewing or editing translation files in the paths above, treat this as a strict quality and safety review.

## Primary goals

- Keep meaning aligned with the source text; do not introduce significant shifts in intent, tone, scope, or policy meaning.
- Keep translations natural and grammatically correct for the target locale.
- If relevant, preserve locale and regional conventions for the target variant (for example `en_US` vs `en_GB`, `en_HK`, `zh_CN` vs `zh_TW` vs `zh_HK`, and other region-specific variants).
- Reject or correct any profanity, slurs, abusive wording, sexual content, harassment, threats, or other potentially offensive/nefarious wording.
- Reject or correct text that could be misleading, manipulative, discriminatory, non-inclusive, or otherwise problematic.

## Source-of-truth alignment

- For `*.arb` locale files, compare translated messages against equivalent keys in `gui/packages/ubuntupro/lib/l10n/app_en.arb`.
- For `*.po` files, compare each `msgstr` against its corresponding `msgid` (or `.pot` source entry).
- Preserve the original intent and user impact level (for example: warning vs info, mandatory vs optional, success vs failure).

## Do-not-change semantics

- Do not add or remove constraints, requirements, legal/security implications, or actionability.
- Do not soften critical warnings or escalate neutral text.
- Do not insert culturally loaded or idiomatic content that changes meaning.
- Do not normalize one regional standard into another when the file/locale indicates a specific dialect or region.

## Technical integrity checks

- Preserve placeholders and variable tokens exactly (for example `{count}`, `{name}`, `%s`, `%d`, ICU/plural/select tokens).
- Preserve markup and escape sequences exactly (for example HTML tags, Markdown links, `\n`).
- Preserve message keys, plural categories, and formatting structure.
- Keep terminology consistent across related strings.

## Review behavior

- If a translation appears to deviate significantly from the source meaning, propose a corrected translation that is closer in meaning.
- If unsure about nuance, state the uncertainty and provide the most literal safe option.
- Prioritize safe, neutral, and professional wording suitable for a broad user audience.
- Prefer region-appropriate spelling, grammar, punctuation, terminology, and date/number conventions for the target locale when those conventions do not alter meaning.
- For each review comment, include a confidence label (`High`, `Medium`, or `Low`); if confidence is `Low`, briefly explain why and avoid definitive wording.
