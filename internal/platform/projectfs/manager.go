package projectfs

import "github.com/lishen802/novel-studio/internal/application/ports"

type Manager struct{}

func (Manager) Create(parentDirectory, projectTitle string) (ports.ProjectPackage, error) {
	layout, err := NewLayout(parentDirectory, projectTitle)
	if err != nil {
		return ports.ProjectPackage{}, err
	}
	if err := layout.Create(); err != nil {
		return ports.ProjectPackage{}, err
	}
	return packageFromLayout(layout), nil
}

func (Manager) Open(packagePath string) (ports.ProjectPackage, error) {
	layout, err := OpenLayout(packagePath)
	if err != nil {
		return ports.ProjectPackage{}, err
	}
	return packageFromLayout(layout), nil
}

func packageFromLayout(layout Layout) ports.ProjectPackage {
	return ports.ProjectPackage{
		Root:        layout.Root,
		Database:    layout.Database,
		Attachments: layout.Attachments,
		Exports:     layout.Exports,
		Imports:     layout.Imports,
		Snapshots:   layout.Snapshots,
		Branches:    layout.Branches,
	}
}

var _ ports.ProjectPackageManager = Manager{}
