package v1alpha1

import (
	"fmt"
	"strings"

	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
)

// FQN (Fully Qualified Name) represents the canonical Resource ID.
// Format: / <kind> / <name> ( / <kind> / <name> )*
// Example: /ws/ws1/prv/prv1/cls/cls1/app/app1/box/api
// Kind shortnames: ws|prv|cls|app|box
type FQN string

// String returns the FQN as a string.
func (f FQN) String() string {
	return string(f)
}

// Segments returns the path segments (excluding the leading slash).
// Example: "/ws/ws1/prv/prv1" -> ["ws", "ws1", "prv", "prv1"]
func (f FQN) Segments() []string {
	s := string(f)
	if s == "" || s == "/" {
		return nil
	}
	// Remove leading slash and split
	s = strings.TrimPrefix(s, "/")
	return strings.Split(s, "/")
}

// KindSegments returns pairs of (kind, name) segments.
// Example: "/ws/ws1/prv/prv1" -> [("ws", "ws1"), ("prv", "prv1")]
func (f FQN) KindSegments() [][2]string {
	segs := f.Segments()
	if len(segs)%2 != 0 {
		return nil
	}
	pairs := make([][2]string, 0, len(segs)/2)
	for i := 0; i < len(segs); i += 2 {
		pairs = append(pairs, [2]string{segs[i], segs[i+1]})
	}
	return pairs
}

// LastKind returns the last kind segment.
func (f FQN) LastKind() string {
	segs := f.Segments()
	if len(segs) < 2 {
		return ""
	}
	return segs[len(segs)-2]
}

// LastName returns the last name segment.
func (f FQN) LastName() string {
	segs := f.Segments()
	if len(segs) < 2 {
		return ""
	}
	return segs[len(segs)-1]
}

// WorkspaceName returns the workspace name.
func (f FQN) WorkspaceName() string {
	segs := f.Segments()
	if len(segs) >= 2 && segs[0] == "ws" {
		return segs[1]
	}
	return ""
}

// ProviderName returns the provider name (if exists).
func (f FQN) ProviderName() string {
	segs := f.Segments()
	if len(segs) >= 4 && segs[2] == "prv" {
		return segs[3]
	}
	return ""
}

// ClusterName returns the cluster name (if exists).
func (f FQN) ClusterName() string {
	segs := f.Segments()
	if len(segs) >= 6 && segs[4] == "cls" {
		return segs[5]
	}
	return ""
}

// AppName returns the app name (if exists).
func (f FQN) AppName() string {
	segs := f.Segments()
	if len(segs) >= 8 && segs[6] == "app" {
		return segs[7]
	}
	return ""
}

// BoxName returns the box name (if exists).
func (f FQN) BoxName() string {
	segs := f.Segments()
	if len(segs) >= 10 && segs[8] == "box" {
		return segs[9]
	}
	return ""
}

// ParentFQN returns the parent FQN.
func (f FQN) ParentFQN() FQN {
	segs := f.Segments()
	if len(segs) <= 2 {
		return ""
	}
	// Remove last kind/name pair
	parentSegs := segs[:len(segs)-2]
	return FQN("/" + strings.Join(parentSegs, "/"))
}

// kindShortToFull maps kind shortnames to full Kind names.
var kindShortToFull = map[string]string{
	"ws":  "Workspace",
	"prv": "Provider",
	"cls": "Cluster",
	"app": "App",
	"box": "Box",
}

// kindFullToShort maps full Kind names to shortnames.
var kindFullToShort = map[string]string{
	"Workspace": "ws",
	"Provider":  "prv",
	"Cluster":   "cls",
	"App":       "app",
	"Box":       "box",
}

// ParseResourceID parses and validates a Resource ID string.
// Format: / <kind> / <name> ( / <kind> / <name> )*
// Returns the FQN and the full Kind name, or an error if invalid.
func ParseResourceID(id string) (FQN, string, error) {
	// Must start with /
	if !strings.HasPrefix(id, "/") {
		return "", "", fmt.Errorf("Resource ID must start with '/': %q", id)
	}

	// Extract segments
	fqn := FQN(id)
	segs := fqn.Segments()

	// Must have even number of segments (kind/name pairs)
	if len(segs)%2 != 0 {
		return "", "", fmt.Errorf("Resource ID must have kind/name pairs: %q", id)
	}

	// Must have at least one pair
	if len(segs) < 2 {
		return "", "", fmt.Errorf("Resource ID must have at least one kind/name pair: %q", id)
	}

	// Validate each pair
	var fullKind string
	for i := 0; i < len(segs); i += 2 {
		kindShort := segs[i]
		name := segs[i+1]

		// Validate kind shortname
		full, ok := kindShortToFull[kindShort]
		if !ok {
			return "", "", fmt.Errorf("unknown kind shortname %q in Resource ID %q", kindShort, id)
		}

		// The last kind is the resource's kind
		if i == len(segs)-2 {
			fullKind = full
		}

		// Validate name as DNS-1123 label
		if errs := utilvalidation.IsDNS1123Label(name); len(errs) > 0 {
			return "", "", fmt.Errorf("invalid name %q in Resource ID %q: %s", name, id, strings.Join(errs, ", "))
		}
	}

	return fqn, fullKind, nil
}

// ValidateResourceID validates a Resource ID for the given Kind.
// Checks:
// 1. Format and parsing
// 2. Last kind segment matches the expected Kind
// 3. Name matches metadata.name
// 4. Parent chain structure is valid
func ValidateResourceID(id string, expectedKind string, expectedName string) (FQN, error) {
	fqn, actualKind, err := ParseResourceID(id)
	if err != nil {
		return "", err
	}

	// Check kind matches
	if actualKind != expectedKind {
		return "", fmt.Errorf("Resource ID kind %q does not match expected kind %q", actualKind, expectedKind)
	}

	// Check name matches
	lastName := fqn.LastName()
	if lastName != expectedName {
		return "", fmt.Errorf("Resource ID name %q does not match metadata.name %q", lastName, expectedName)
	}

	// Validate parent chain structure
	if err := validateParentChain(fqn, expectedKind); err != nil {
		return "", err
	}

	return fqn, nil
}

// validateParentChain validates that the parent chain is structurally correct.
// Example: Box must have parent chain: /ws/.../prv/.../cls/.../app/...
func validateParentChain(fqn FQN, kind string) error {
	pairs := fqn.KindSegments()
	if len(pairs) == 0 {
		return fmt.Errorf("invalid Resource ID structure")
	}

	// Expected parent chain for each kind
	expectedChains := map[string][]string{
		"Workspace": {},
		"Provider":  {"ws"},
		"Cluster":   {"ws", "prv"},
		"App":       {"ws", "prv", "cls"},
		"Box":       {"ws", "prv", "cls", "app"},
	}

	expectedChain, ok := expectedChains[kind]
	if !ok {
		return fmt.Errorf("unknown kind: %s", kind)
	}

	// Check that we have the right number of pairs
	if len(pairs) != len(expectedChain)+1 {
		return fmt.Errorf("kind %s expects %d kind/name pairs, got %d in Resource ID %q", kind, len(expectedChain)+1, len(pairs), fqn)
	}

	// Validate parent chain kinds
	for i, expectedKind := range expectedChain {
		if pairs[i][0] != expectedKind {
			return fmt.Errorf("expected kind %q at position %d in Resource ID %q, got %q", expectedKind, i, fqn, pairs[i][0])
		}
	}

	// Validate the resource's own kind
	lastKind := pairs[len(pairs)-1][0]
	expectedLastKind, ok := kindFullToShort[kind]
	if !ok {
		return fmt.Errorf("unknown kind: %s", kind)
	}
	if lastKind != expectedLastKind {
		return fmt.Errorf("expected kind shortname %q for %s, got %q in Resource ID %q", expectedLastKind, kind, lastKind, fqn)
	}

	return nil
}

// BuildResourceID constructs a Resource ID from a parent ID and kind/name.
func BuildResourceID(parentID FQN, kind string, name string) (FQN, error) {
	// Validate name
	if errs := utilvalidation.IsDNS1123Label(name); len(errs) > 0 {
		return "", fmt.Errorf("invalid name %q: %s", name, strings.Join(errs, ", "))
	}

	// Get kind shortname
	kindShort, ok := kindFullToShort[kind]
	if !ok {
		return "", fmt.Errorf("unknown kind: %s", kind)
	}

	var fqn FQN
	if kind == "Workspace" {
		// Workspace has no parent
		if parentID != "" {
			return "", fmt.Errorf("Workspace cannot have a parent")
		}
		fqn = FQN(fmt.Sprintf("/ws/%s", name))
	} else {
		// Other kinds require a parent
		if parentID == "" {
			return "", fmt.Errorf("kind %s requires a parent Resource ID", kind)
		}
		fqn = FQN(fmt.Sprintf("%s/%s/%s", parentID, kindShort, name))
	}

	// Validate the constructed ID
	_, err := ValidateResourceID(string(fqn), kind, name)
	if err != nil {
		return "", fmt.Errorf("constructed invalid Resource ID: %w", err)
	}

	return fqn, nil
}

// ExtractResourceID extracts and validates the Resource ID from annotations.
func ExtractResourceID(kind string, name string, annotations map[string]string) (FQN, error) {
	id, ok := annotations[AnnotationID]
	if !ok {
		return "", fmt.Errorf("missing required annotation %q", AnnotationID)
	}

	return ValidateResourceID(id, kind, name)
}
