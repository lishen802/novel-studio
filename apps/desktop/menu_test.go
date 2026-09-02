package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/wailsapp/wails/v2/pkg/menu"
)

func TestMenuCommandWireContractUsesType(t *testing.T) {
	encoded, err := json.Marshal(MenuCommand{Command: menuCmdOpenProject})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(encoded), `{"type":"open_project"}`; got != want {
		t.Fatalf("menu command JSON = %s, want %s", got, want)
	}
}

func TestQuitMenuEmitsApplicationQuitInsteadOfProjectClose(t *testing.T) {
	var emitted string
	model := buildMenu(func(command string) { emitted = command })
	appMenu := model.Items[0]
	for _, item := range appMenu.SubMenu.Items {
		if item.Label != "退出织境" {
			continue
		}
		if item.Click == nil {
			t.Fatal("退出织境没有点击回调")
		}
		item.Click(&menu.CallbackData{MenuItem: item})
		if emitted != "quit_application" {
			t.Fatalf("退出织境发送 %q，期望 quit_application", emitted)
		}
		return
	}
	t.Fatal("未找到退出织境菜单项")
}

// menuLabels returns the labels of all non-separator items in a submenu.
func menuLabels(m *menu.Menu) []string {
	if m == nil {
		return nil
	}
	var labels []string
	for _, item := range m.Items {
		if item.IsSeparator() {
			continue
		}
		labels = append(labels, item.Label)
	}
	return labels
}

// findSubmenu returns the submenu with the given label from a menu's top-level
// items, or nil if not found.
func findSubmenu(m *menu.Menu, label string) *menu.Menu {
	if m == nil {
		return nil
	}
	for _, item := range m.Items {
		if item.Label == label {
			return item.SubMenu
		}
	}
	return nil
}

// assertMenuContains fails the test if any of the expected labels are missing
// from the menu.
func assertMenuContains(t *testing.T, m *menu.Menu, expected ...string) {
	t.Helper()
	labels := menuLabels(m)
	for _, want := range expected {
		found := false
		for _, got := range labels {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("menu missing %q; has %v", want, labels)
		}
	}
}

// assertMenuOmits fails the test if any of the unwanted labels are present in
// the menu.
func assertMenuOmits(t *testing.T, m *menu.Menu, unwanted ...string) {
	t.Helper()
	labels := menuLabels(m)
	for _, bad := range unwanted {
		for _, got := range labels {
			if got == bad {
				t.Fatalf("menu should not contain %q; has %v", bad, labels)
			}
		}
	}
}

func TestApplicationMenuContainsOnlyApplicationCommands(t *testing.T) {
	model := buildMenuModel()
	if len(model.Items) == 0 {
		t.Fatal("menu has no items")
	}

	// The first item is the application menu (AppMenu role on macOS).
	appMenu := model.Items[0]
	if appMenu.SubMenu == nil {
		t.Fatal("application menu has no submenu")
	}

	assertMenuContains(t, appMenu.SubMenu, "关于织境", "设置…", "检查更新…", "隐藏织境", "退出织境")
	assertMenuOmits(t, appMenu.SubMenu, "新建原创作品…", "导入小说并续写…", "打开项目…", "关闭当前项目", "项目设置…", "导出…", "项目维护…")
}

func TestFileMenuContainsProjectLifecycleCommands(t *testing.T) {
	model := buildMenuModel()
	fileMenu := findSubmenu(model, "文件")
	if fileMenu == nil {
		t.Fatal("file menu not found")
	}

	assertMenuContains(t, fileMenu,
		"新建原创作品…",
		"导入小说并续写…",
		"打开项目…",
		"打开最近项目",
		"关闭当前项目",
		"项目设置…",
		"导出…",
		"项目维护…",
	)
}

func TestFileMenuContainsRecentProjectsSubmenu(t *testing.T) {
	model := buildMenuModel()
	fileMenu := findSubmenu(model, "文件")
	if fileMenu == nil {
		t.Fatal("file menu not found")
	}

	for _, item := range fileMenu.Items {
		if item.Label == "打开最近项目" {
			if item.SubMenu == nil {
				t.Fatal("打开最近项目 has no submenu")
			}
			return
		}
	}
	t.Fatal("打开最近项目 item not found in file menu")
}

func TestViewMenuContainsToggleCommands(t *testing.T) {
	model := buildMenuModel()
	viewMenu := findSubmenu(model, "显示")
	if viewMenu == nil {
		t.Fatal("view menu not found")
	}

	assertMenuContains(t, viewMenu, "切换导航面板", "切换 AI 面板", "聚焦 AI 输入")
}

func TestMenuContainsEditAndWindowRoles(t *testing.T) {
	model := buildMenuModel()

	foundEdit := false
	foundWindow := false
	for _, item := range model.Items {
		if item.Role == menu.EditMenuRole {
			foundEdit = true
			if item.Label != "编辑" {
				t.Errorf("编辑菜单标题 = %q，期望 编辑", item.Label)
			}
		}
		if item.Role == menu.WindowMenuRole {
			foundWindow = true
			if item.Label != "窗口" {
				t.Errorf("窗口菜单标题 = %q，期望 窗口", item.Label)
			}
		}
	}
	if !foundEdit {
		t.Fatal("menu missing EditMenu role")
	}
	if !foundWindow {
		t.Fatal("menu missing WindowMenu role")
	}
}

func TestMenuCommandValuesAreStable(t *testing.T) {
	// These command values are part of the stable contract with the frontend.
	// They must never change without a coordinated frontend update.
	expected := []string{
		menuCmdNewOriginal,
		menuCmdImportNovel,
		menuCmdOpenProject,
		menuCmdCloseProject,
		menuCmdAppSettings,
		menuCmdProjectSettings,
		menuCmdToggleNavigation,
		menuCmdToggleAIPanel,
		menuCmdFocusAI,
	}
	for _, cmd := range expected {
		if strings.TrimSpace(cmd) == "" {
			t.Fatalf("empty menu command value")
		}
	}
	if menuCmdOpenRecentPrefix != "open_recent:" {
		t.Fatalf("open_recent prefix = %q, want %q", menuCmdOpenRecentPrefix, "open_recent:")
	}
}
