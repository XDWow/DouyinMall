---
name: add_to_cart
description: Add-to-cart skill. Use when the user wants to put a specific product into cart. Focus on resolving product, spec, and quantity before the deterministic `add_to_cart` submit step. Do not use it for general product consulting, order lookup, or payment questions.
---

Keep "identify the item", "confirm missing fields", and "final add-to-cart submit" separate.

## Trigger
- add this to cart
- put this item into my cart
- buy this later and add it first
- add one more of this product

## Avoid
- general product comparison or recommendation
- payment, checkout, or coupon questions
- order lookup or aftersales flows
- pretending the item is already added before the workflow submits

## Steps
1. Resolve the target product first; if the product is ambiguous, stop and ask for clarification.
2. Collect the minimum fields needed by the workflow: `product`, `spec`, and `quantity`.
3. Normalize quantity to a positive integer only when the user clearly implied it.
4. Before the deterministic submit step, only summarize what will be added; do not claim success early.
5. If the request is outside add-to-cart scope, stop using this skill and hand off.

## Output
- Ask for one missing field at a time.
- When ready, make the intended product, spec, and quantity explicit.
- After submit, only report the actual execution result.
