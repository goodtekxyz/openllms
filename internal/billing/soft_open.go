package billing

// PublicPaidCheckout is false during Free-only soft-open.
// Checkout handlers and public plan CTAs must stay off while this is false.
// Polar/Unifi code paths remain for a later re-open; do not delete them.
const PublicPaidCheckout = false

// SoftOpen is true when the product ships Free + soft-cap only (no public paid SKUs).
const SoftOpen = !PublicPaidCheckout
