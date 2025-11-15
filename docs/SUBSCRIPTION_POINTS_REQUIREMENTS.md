# Subscription Points Requirements

Recent backend refactors removed the legacy fallback that tried to read `SubscriptionPoints` from a package whenever a subscription did not have its own point allocation. Every subscription **must now be provisioned with an explicit `SubscriptionPoints` value** during creation (both manual creates and package-based creates already expose this field in their payloads).

Key implications:

- Group access creation and updates will reject requests when the owning subscription has `SubscriptionPoints <= 0`. Set the desired allowance before creating groups.
- Instructor dashboards pull usage and remaining points directly from the subscription record. Incorrect or missing point balances will surface there immediately.
- Migration scripts or bootstrap routines that seed subscriptions should pass the intended point totals explicitly instead of relying on package defaults.

Keep this in mind when syncing with the Flutter app or any automation that provisions subscriptions so that the new validation layer works as expected.
