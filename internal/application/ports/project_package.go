package ports

type ProjectPackage struct {
	Root        string
	Database    string
	Attachments string
	Exports     string
	Imports     string
	Snapshots   string
	Branches    string
}

type ProjectPackageManager interface {
	Create(parentDirectory, projectTitle string) (ProjectPackage, error)
	Open(packagePath string) (ProjectPackage, error)
}
