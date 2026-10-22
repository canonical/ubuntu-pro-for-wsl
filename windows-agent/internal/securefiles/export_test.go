package securefiles

// NewCustodianAtBasePath stands in for Open when the sub-tree's name cannot be
// established on the testing machine, such as a UNC share or an NT-prefixed form
// that would have to be created as literal directories on Linux: it hands the
// custodian the base path with nothing established. CheckProjection needs no
// established tree to classify a volume, only the name.
func NewCustodianAtBasePath(basePath string) *Custodian {
	return &Custodian{basePath: basePath}
}
