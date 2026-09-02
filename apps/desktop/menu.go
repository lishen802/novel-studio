package main

import (
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const menuCommandEvent = "app:menu-command"

// MenuCommand 是原生菜单发给前端的稳定事件载荷，线上的 type 字段与前端契约一致。
type MenuCommand struct {
	Command string `json:"type"`
}

// Stable menu command values. React matches these exactly; they must never
// change without a coordinated frontend update.
const (
	menuCmdNewOriginal        = "new_original"
	menuCmdImportNovel        = "import_novel"
	menuCmdOpenProject        = "open_project"
	menuCmdCloseProject       = "close_project"
	menuCmdQuitApplication    = "quit_application"
	menuCmdAppSettings        = "app_settings"
	menuCmdProjectSettings    = "project_settings"
	menuCmdToggleNavigation   = "toggle_navigation"
	menuCmdToggleAIPanel      = "toggle_ai_panel"
	menuCmdFocusAI            = "focus_ai"
	menuCmdCheckForUpdates    = "check_for_updates"
	menuCmdExport             = "export"
	menuCmdProjectMaintenance = "project_maintenance"
	menuCmdAbout              = "about"
)

// menuCommandPrefixes are used to build dynamic commands like
// "open_recent:<project-id>".
const menuCmdOpenRecentPrefix = "open_recent:"

// buildMenuModel returns the native menu structure with nil click callbacks.
// It is the testable surface for asserting menu labels and grouping; the App
// attaches real callbacks via buildMenu.
func buildMenuModel() *menu.Menu {
	return buildMenu(nil)
}

// buildMenu constructs the full native menu. When emit is non-nil, each
// business menu item gets a click callback that emits its stable command.
// When emit is nil (used by buildMenuModel for testing), items have no
// callbacks.
func buildMenu(emit func(string)) *menu.Menu {
	appMenu := menu.AppMenu()
	appMenu.Label = "织境"
	appMenu.SubMenu = buildApplicationMenu(emit)

	fileMenu := menu.NewMenuFromItems(
		menu.Text("新建原创作品…", nil, clickFor(emit, menuCmdNewOriginal)),
		menu.Text("导入小说并续写…", nil, clickFor(emit, menuCmdImportNovel)),
		menu.Separator(),
		menu.Text("打开项目…", nil, clickFor(emit, menuCmdOpenProject)),
		menu.SubMenu("打开最近项目", buildRecentProjectsMenu(emit)),
		menu.Separator(),
		menu.Text("关闭当前项目", nil, clickFor(emit, menuCmdCloseProject)),
		menu.Text("项目设置…", nil, clickFor(emit, menuCmdProjectSettings)),
		menu.Separator(),
		menu.Text("导出…", nil, clickFor(emit, menuCmdExport)),
		menu.Text("项目维护…", nil, clickFor(emit, menuCmdProjectMaintenance)),
	)

	viewMenu := menu.NewMenuFromItems(
		menu.Text("切换导航面板", nil, clickFor(emit, menuCmdToggleNavigation)),
		menu.Text("切换 AI 面板", nil, clickFor(emit, menuCmdToggleAIPanel)),
		menu.Text("聚焦 AI 输入", nil, clickFor(emit, menuCmdFocusAI)),
	)

	helpMenu := menu.NewMenuFromItems(
		menu.Text("查看使用文档", nil, clickFor(emit, menuCmdAbout)),
		menu.Separator(),
		menu.Text("报告问题…", nil, nil),
	)
	editMenu := menu.EditMenu()
	editMenu.Label = "编辑"
	windowMenu := menu.WindowMenu()
	windowMenu.Label = "窗口"

	return menu.NewMenuFromItems(
		appMenu,
		menu.SubMenu("文件", fileMenu),
		editMenu,
		menu.SubMenu("显示", viewMenu),
		windowMenu,
		menu.SubMenu("帮助", helpMenu),
	)
}

// buildApplicationMenu creates the application-level submenu (About, Settings,
// Check for Updates, Hide, Quit). On macOS this submenu is attached to the
// AppMenu role item so the system renders it as the application menu.
func buildApplicationMenu(emit func(string)) *menu.Menu {
	return menu.NewMenuFromItems(
		menu.Text("关于织境", nil, clickFor(emit, menuCmdAbout)),
		menu.Separator(),
		menu.Text("设置…", nil, clickFor(emit, menuCmdAppSettings)),
		menu.Text("检查更新…", nil, clickFor(emit, menuCmdCheckForUpdates)),
		menu.Separator(),
		menu.Text("隐藏织境", nil, nil),
		menu.Text("退出织境", nil, clickFor(emit, menuCmdQuitApplication)),
	)
}

// buildRecentProjectsMenu creates the "打开最近项目" submenu. It is empty in
// the static model; App.menu populates it from the settings store before
// returning the menu to Wails.
func buildRecentProjectsMenu(emit func(string)) *menu.Menu {
	m := menu.NewMenu()
	if emit == nil {
		return m
	}
	// App.menu populates recent items dynamically.
	return m
}

// clickFor returns a menu callback that emits the given command via the emit
// function. When emit is nil, it returns nil so the menu item has no callback.
func clickFor(emit func(string), command string) menu.Callback {
	if emit == nil {
		return nil
	}
	return func(*menu.CallbackData) {
		emit(command)
	}
}

// menu returns the native menu for the Wails application. It builds the static
// structure and populates the "打开最近项目" submenu from the settings store.
func (a *App) menu() *menu.Menu {
	m := buildMenu(a.emitMenuCommand)
	a.populateRecentProjects(m)
	return m
}

// populateRecentProjects fills the "打开最近项目" submenu with recent project
// entries from the settings store. It is best-effort: if the settings store is
// unavailable or empty, the submenu remains empty.
func (a *App) populateRecentProjects(m *menu.Menu) {
	if a.api == nil || a.api.settings == nil {
		return
	}
	recent, err := a.api.settings.RecentProjects(a.api.ctx, 20)
	if err != nil || len(recent) == 0 {
		return
	}
	for _, item := range m.Items {
		if item.Label != "文件" || item.SubMenu == nil {
			continue
		}
		for _, fileItem := range item.SubMenu.Items {
			if fileItem.Label == "打开最近项目" && fileItem.SubMenu != nil {
				for _, rp := range recent {
					projectID := rp.ProjectID
					label := rp.Title
					if rp.Missing {
						label += "（已失效）"
					}
					fileItem.SubMenu.AddText(label, nil, func(*menu.CallbackData) {
						a.emitMenuCommand(menuCmdOpenRecentPrefix + projectID)
					})
				}
				return
			}
		}
	}
}

// emitMenuCommand emits a stable menu command event to the frontend via Wails
// runtime events. React subscribes to menuCommandEvent and dispatches on the
// Command field.
func (a *App) emitMenuCommand(command string) {
	if a.api == nil || a.api.ctx == nil {
		return
	}
	runtime.EventsEmit(a.api.ctx, menuCommandEvent, MenuCommand{Command: command})
}
