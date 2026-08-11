package projectfs

import "github.com/lishen802/novel-studio/internal/application/ports"

type Manager struct {
	// recoveryCopyHooks is test-only dependency injection. Production callers
	// use the zero value and therefore the durable filesystem operations below.
	recoveryCopyHooks *recoveryCopyHooks
}

func (Manager) Create(parentDirectory, projectTitle string) (ports.ProjectPackage, error) {
	if err := (Manager{}).ValidateCreatePath(parentDirectory, projectTitle); err != nil {
		return ports.ProjectPackage{}, err
	}
	layout, err := NewLayout(parentDirectory, projectTitle)
	if err != nil {
		return ports.ProjectPackage{}, err
	}
	if err := layout.Create(); err != nil {
		return ports.ProjectPackage{}, err
	}
	return packageFromLayout(layout), nil
}

func (Manager) Inspect(packagePath string) (ports.ProjectPackage, error) {
	layout, err := InspectLayout(packagePath)
	if err != nil {
		return ports.ProjectPackage{}, err
	}
	return packageFromLayout(layout), nil
}

func (Manager) PrepareEditable(packageInfo ports.ProjectPackage) error {
	return PrepareEditableLayout(layoutForRoot(packageInfo.Root))
}

func packageFromLayout(layout Layout) ports.ProjectPackage {
	return ports.ProjectPackage{
		Root:        layout.Root,
		Database:    layout.Database,
		Attachments: layout.Attachments,
		Exports:     layout.Exports,
		Imports:     layout.Imports,
		Recovery:    layout.Recovery,
		Snapshots:   layout.Snapshots,
		Branches:    layout.Branches,
		Lease:       layout.Lease,
	}
}

var _ ports.ProjectPackageManager = Manager{}
