package lyresdk

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var identifier = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,99}$`)
var capabilityName = regexp.MustCompile(`^[a-z][a-z0-9._-]{1,99}$`)

// Reference is the canonical logical reference a caller sends to Lyre.
type Reference struct {
	Capability           string
	ProviderID           string
	ProviderCapabilityID string
	ContractVersion      uint32
}

func (r Reference) String() string {
	base := "lyre." + r.Capability
	if r.ProviderID != "" {
		base += "@" + r.ProviderID + "." + r.ProviderCapabilityID
	}
	if r.ContractVersion != 0 {
		base += "@v" + strconv.FormatUint(uint64(r.ContractVersion), 10)
	}
	return base
}

// ParseReference accepts canonical lyre.family[@provider.implementation][@vN] references.
func ParseReference(value string) (Reference, error) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "lyre.") {
		return Reference{}, fmt.Errorf("capability reference must begin with lyre.")
	}
	parts := strings.Split(value[len("lyre."):], "@")
	if len(parts) < 1 || len(parts) > 3 || !capabilityName.MatchString(parts[0]) {
		return Reference{}, fmt.Errorf("invalid capability reference")
	}
	ref := Reference{Capability: parts[0]}
	for _, part := range parts[1:] {
		if strings.HasPrefix(part, "v") && !strings.Contains(part, ".") {
			if ref.ContractVersion != 0 {
				return Reference{}, fmt.Errorf("contract version is repeated")
			}
			version, err := strconv.ParseUint(strings.TrimPrefix(part, "v"), 10, 32)
			if err != nil || version == 0 {
				return Reference{}, fmt.Errorf("invalid contract version")
			}
			ref.ContractVersion = uint32(version)
			continue
		}
		if ref.ProviderID != "" || ref.ContractVersion != 0 {
			return Reference{}, fmt.Errorf("provider capability is repeated")
		}
		providerID, providerCapabilityID, ok := strings.Cut(part, ".")
		if !ok || !identifier.MatchString(providerID) || !identifier.MatchString(providerCapabilityID) {
			return Reference{}, fmt.Errorf("invalid provider capability")
		}
		ref.ProviderID, ref.ProviderCapabilityID = providerID, providerCapabilityID
	}
	return ref, nil
}
