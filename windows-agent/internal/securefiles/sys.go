package securefiles

import "os"

// stamp is the ownership mark the custodian writes on every node it creates:
// the owner the node projects as inside an instance (root) and the strict
// modes. The same values on every platform; only the carrier differs — NT
// extended attributes on Windows, a user-namespace extended attribute off it.
type stamp struct {
	uid  uint32
	gid  uint32
	mode uint32
}

// fileStamp is the stamp every created file carries: a regular file, owned
// by root, mode 0600. The file-type bits ride the mode, as they do in the
// projection.
func fileStamp() stamp {
	return stamp{uid: 0, gid: 0, mode: 0100600}
}

// fileIdentity is a node's identity as its filesystem reports it: what ties
// the tree the custodian established to whatever its path resolves to a
// moment later. Opaque data both platforms fill; only the custodian compares.
type fileIdentity struct {
	volume uint64
	file   [16]byte
}

// openProfile is the access a caller needs the node opened with: openRead
// answers questions about the node, openRename is the profile the
// publish-by-rename path needs, where the platform renames through the
// descriptor, so that the verification and the rename answer for the same
// node; openAppend is the append-only profile, whose writes go to the end of
// the node on the kernel's own authority; openDir lists a directory. Every
// profile opens the node itself, never the target of a link.
type openProfile int

const (
	openRead openProfile = iota
	openRename
	openAppend
	openDir
)

// platformSys is the custodian's OS layer: one method per mechanism, no
// policy. Every decision that can be made without knowing the mechanism lives
// in custodian.go; what remains here is a syscall, or a compound whose parts
// cannot be interleaved without losing a security property the custodian
// promises — the stamp riding the creating syscall, the no-reparse open, the
// root held without sharing deletion.
type platformSys interface {
	// establishRoot creates or adopts the tree root at the path the layer was
	// constructed with, stamped either way, and holds it for the custodian's
	// lifetime where the platform can refuse a rename of an open root
	// (Windows: the stamping open declines to share deletion, so nothing else
	// can rename or remove the tree while the custodian serves; POSIX cannot
	// refuse a rename because a descriptor is open, and the runtime root-swap
	// window stays closed by instance-side validation instead, ADR 2.01). A
	// root it cannot stamp is refused, not served unstamped
	// (ErrNoWatermarkSupport); one a link stands in for is refused rather than
	// followed (ErrPathEscapes). It answers the identity it established, and
	// whether the filesystem supplied a usable one.
	establishRoot() (id fileIdentity, known bool, err error)

	// sub returns the layer for a sub-custodian: the same surface as the
	// parent's, as it stands at the handout — a seam installed on the parent
	// before Subdir governs the sub-tree too, one installed after does not
	// reach an already-handed-out child. A sub-tree root never resolves a path
	// of its own: it is reached through the parent's containment.
	sub() platformSys

	// attach derives the operation base from a root the custodian opened and
	// verified. Where the platform holds the root, the hold is taken here,
	// beside the verified node.
	attach(root *os.Root) error

	// release releases everything the layer holds for the custodian's
	// lifetime. The custodian closes the os.Root itself.
	release() error

	// identity answers the filesystem identity of an open node, and whether
	// the filesystem supplied a usable one. A filesystem that supplies no
	// identity is not an error; it only means the node cannot be
	// re-identified later.
	identity(f *os.File) (fileIdentity, bool)

	// createStamped creates rel under the tree root, stamped as the
	// custodian's own, with no window between the node's existence and its
	// stamp where the platform can close both in one syscall. A failure
	// leaves no node behind. os.ErrExist answers an adoption the caller
	// (Subdir) decides about.
	createStamped(rel string, dir bool) (*os.File, error)

	// stampDir stamps an adopted directory in place. ADR 2.01 requires
	// first-level sub-tree roots to carry the stamp even when pre-existing,
	// because it is what revokes unprivileged creation and deletion inside
	// them.
	stampDir(rel string) error

	// openNode opens the node standing at rel with the profile's access,
	// never the target of a reparse point: a link standing there answers
	// ErrPathEscapes. The append profile opens without write-the-middle
	// rights, so a descriptor on it can only ever append; the directory
	// profile answers for a directory. A node that is not the kind the
	// profile expects answers the platform's refusal for that mismatch.
	openNode(rel string, profile openProfile) (*os.File, error)

	// readStamp reads the ownership stamp of an open node. ok=false is the
	// verdict "this node carries none", not a failure to read one; any other
	// failure cannot answer and is returned as the error.
	readStamp(f *os.File) (s stamp, ok bool, err error)

	// renameReplace renames the open node, standing at oldRel, to newRel
	// under the tree root, replacing whatever stands there. posix selects
	// POSIX semantics — an open destination does not refuse the rename — and
	// the degraded class serves volumes that predate them.
	renameReplace(f *os.File, oldRel, newRel string, posix bool) error

	// isRenameClassUnsupported answers whether err says the volume lacks the
	// POSIX rename class, so retrying with the degraded one is worthwhile.
	// False everywhere but Windows.
	isRenameClassUnsupported(err error) bool

	// isLink answers whether a reparse point stands at rel, opening the point
	// itself, never its target, so every link kind is classified uniformly.
	isLink(rel string) (bool, error)

	// finalPath answers the tree root's path as the filesystem finally names
	// it, following any links in the path the custodian was handed, or ""
	// when the answer is unavailable.
	finalPath() string
}
