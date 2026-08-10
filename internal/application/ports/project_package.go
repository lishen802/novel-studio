package ports

type ProjectPackage struct {
	Root        string
	Database    string
	Attachments string
	Exports     string
	Imports     string
	Recovery    string
	Snapshots   string
	Branches    string
	Lease       string
}

type ProjectPackageManager interface {
	Create(parentDirectory, projectTitle string) (ProjectPackage, error)
	Inspect(packagePath string) (ProjectPackage, error)
	PrepareEditable(packageInfo ProjectPackage) error
}
