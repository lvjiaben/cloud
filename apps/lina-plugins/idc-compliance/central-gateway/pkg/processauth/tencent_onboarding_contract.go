// This file defines the exact signed route and scope contract for Tencent onboarding.

package processauth

const (
	// TencentOnboardingPath exposes only the buyer installation's safe activation state.
	TencentOnboardingPath = "/internal/v1/tencent-essbasic/onboarding"
	// TencentOnboardingLinksPath creates one official, one-use Tencent console link.
	TencentOnboardingLinksPath = "/internal/v1/tencent-essbasic/onboarding-links"
	// TencentOnboardingRechecksPath queries Tencent's authoritative activation flags.
	TencentOnboardingRechecksPath = "/internal/v1/tencent-essbasic/onboarding-rechecks"
	// TencentOnboardingReadScope authenticates only the safe local status read.
	TencentOnboardingReadScope = "tencent-essbasic.onboarding.read"
	// TencentOnboardingLinkScope authenticates only official link creation.
	TencentOnboardingLinkScope = "tencent-essbasic.onboarding.link"
	// TencentOnboardingRecheckScope authenticates only an explicit authoritative recheck.
	TencentOnboardingRecheckScope = "tencent-essbasic.onboarding.recheck"
)
