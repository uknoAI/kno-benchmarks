# Citing a number from this repository

`uknoAI/kno` ships a document called *What the numbers mean*, which sets out what a confidence
interval claims, why a dev-set estimate is inflated, and what a cost figure does and does not
assert about your data. This file applies the same discipline to numbers about **kno itself**. A
project that refuses to report a delta without its interval and then publishes "10× faster" with
no n and no spread has a credibility problem it built for itself.

## Right now, there is nothing to cite

`results/latest.json` carries an empty `citable_figures` array and a `status` of
`no-citable-figures`. Every figure in `SUMMARY.md` is marked not citable, with the reason attached.
If you are looking for a number to put on a page, the correct outcome today is **no number**. "No
number" is an acceptable outcome. "An illustration where a reader expects a measurement" is not.

## The mechanism, for when there is something to cite

Every figure carries a non-empty `claim` string containing its full qualification. **A consumer is
required to render the qualification adjacent to the number.** The claim travels *inside* the data
so a renderer has to actively discard it rather than merely forget it — and no schema can force a
renderer to display a string, which is a residual risk this file states rather than hides.

Cite a **version-pinned** figure, never "latest". A published page's claim must not silently change
underneath it.

## May claim

- "Driving `value` over 1 000 Assets and 500 Cases with `fake:` at concurrency 8 took a median of
  X s (IQR [a, b], n=7) on `<named machine>`, kno v0.1.2." — every component present: what, at what
  settings, on what machine, with what spread, at what version.
- "Peak RSS stayed under X MiB while streaming 1 000 000 Cases through `baseline`."
- A fitted scaling exponent **with its residuals shown**, naming the base cell the axis was varied
  from.
- A dollar figure **with its pricing-table date**, as an estimate and not an invoice — "reported
  usage at rates as published on `<date>`" — with n and a range.

## May not claim

- **Any comparison to another tool.** Non-negotiable. We do not run their code on our machine, and
  a competitor benchmark written by us is marketing.
- "Fast", "faster", "blazing", "scales linearly" — without the fitted exponent and its residuals
  these are adjectives, not measurements.
- Any figure from an `unstable`-flagged cell without the flag rendered next to it.
- **Any number without n and a spread.** A mean with no spread does not ship.
- Extrapolation past the measured range. N=1 000 was measured; N=10 000 was not, and the curve is
  not an invitation.
- **A `fake:` number presented as an end-to-end time.** `fake:` excludes provider latency, which
  dominates every real run. Every `fake:` figure's claim string contains "excludes provider
  latency", and the schema check fails the build if one does not.
- Any statement of the form "across the matrix". The sweep varies one axis at a time from a base
  cell; a claim about the interior of the space is a claim about something nobody measured.
- Any dollar figure without the pricing-table date.
