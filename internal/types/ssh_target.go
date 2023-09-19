package types

import "regexp"

// SshTarget specifies the target K8s object to route the SSH session to.
type SshTarget struct {
	Namespace string
	Pod       string
	Container string
}

// Regular expression to extract target object hints from the login string
// supplied by the user for SSH connection.
// The format is namespace?:pod?:container?
var hintsRe = regexp.MustCompile(
	`^(?P<ns>[^:]*)?:(?P<pod>[^:]*)?:(?P<container>[^:]*)?$`,
)

// InitFromUsername configures the object to assign target "hints" extracted
// from the user-supplied login string.
//
// If the user supplied namespace?:pod?:container? in the login part of the
// connection string as a hint, relevant values will be assigned to the fields
// of this SshTarget object. The corresponding fields remains empty if the
// username doesn't contain hint information.
func (s *SshTarget) InitFromUsername(username string) {

	matches := hintsRe.FindStringSubmatch(username)
	if len(matches) == 0 {
		return
	}

	idxNs := hintsRe.SubexpIndex("ns")
	idxPod := hintsRe.SubexpIndex("pod")
	idxContainer := hintsRe.SubexpIndex("container")

	s.Namespace = matches[idxNs]
	s.Pod = matches[idxPod]
	s.Container = matches[idxContainer]
}

// IsComplete returns true if all components of the target are known
func (s SshTarget) IsComplete() bool {
	return s.Namespace != "" && s.Pod != "" && s.Container != ""
}
