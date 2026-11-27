package main

import (
	_ "embed"
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/service"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/settings"
)

//go:embed resources/icon.png
var iconPNG []byte

//go:embed resources/icon-tray.png
var trayPNG []byte

const (
	windowTitle = "Mile Knowledge Base"
	// background equals --raw-canvas in both themes (PRD Q6).
	background = "light-dark(#eceef1, #0e0f11)"
)

// legacyDataDir is the old Electron app's userData directory. Only the one-shot
// settings import reads it (PLAN-M0 V-SET-2); tests never call this.
func legacyDataDir() string {
	appData, err := os.UserConfigDir() // %APPDATA% on Windows
	if err != nil {
		return ""
	}
	return filepath.Join(appData, "mile-knowledge-base")
}

func main() {
	dataDir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		log.Fatal(err)
	}
	app, err := service.New(dataDir, legacyDataDir())
	if err != nil {
		log.Fatal(err)
	}
	// The about page reports the packaged version (M74); empty in dev builds.
	app.Version = mygo.App.Version()

	// Bind before Run: `mygo generate` enumerates the bindings the app makes at
	// startup, so services registered later would be missing from the TS client.
	vaults := service.NewVaultService(app)
	settingsSvc := service.NewSettingsService(app)
	docs := service.NewDocService(app)
	renders := service.NewRenderService(app)
	windows := service.NewWindowService(app, vaults)
	indexes := service.NewIndexService(app)
	mutations := service.NewMutationService(app)
	agents := service.NewAgentService(app)
	conversations := service.NewConversationService(app)
	skillSvc := service.NewSkillService(app)
	mygo.Bind(vaults)
	mygo.Bind(settingsSvc)
	mygo.Bind(docs)
	mygo.Bind(renders)
	mygo.Bind(windows)
	mygo.Bind(indexes)
	mygo.Bind(mutations)
	mygo.Bind(agents)
	mygo.Bind(conversations)
	mygo.Bind(skillSvc)

	// Schemes must be registered before the windows that use them.
	if err := mygo.Protocol.HandleFunc("kbimg", service.ImageHandler(app)); err != nil {
		log.Println("kbimg protocol:", err)
	}

	// One-shot import and the saved theme happen before the first paint.
	imported := app.Settings.ImportLegacy()
	applyTheme(app.Settings.Get().Theme)
	// 主题色经 PreloadScript 在页面脚本之前注入，首帧就是所选颜色（技术设计 14.2）。
	accentJSON, err := json.Marshal(app.Settings.Get().Accent)
	if err != nil {
		log.Fatal(err)
	}
	// 外观档位走同一条链路：首帧就按所选动画 / 模糊档位绘制（技术设计 21.2）。
	effectsJSON, err := json.Marshal(app.Settings.Get().Effects)
	if err != nil {
		log.Fatal(err)
	}
	appearancePreload := "window.__MKB_ACCENT__ = " + string(accentJSON) + ";\n" +
		"window.__MKB_EFFECTS__ = " + string(effectsJSON) + ";"

	mygo.App.WhenReady(func() {
		startUI(app, docs, imported, appearancePreload)
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

func startUI(app *service.App, docs *service.DocService, imported settings.ImportResult, appearancePreload string) {
	win := mygo.NewWindow(mygo.WindowOptions{
		Title:           windowTitle,
		Width:           1440,
		Height:          900,
		MinWidth:        744,
		MinHeight:       640,
		Frameless:       true,
		BackgroundColor: background,
		StateKey:        "main",
		URL:             "/",
		Page:            mygo.PageOptions{PreloadScript: appearancePreload},
	})
	app.SetWindow(win)
	installCloseGuard(app, win)
	installTitleBarEvents(win)
	watchTheme(app, win)

	if imported.Ran && imported.Reason != "" {
		_ = service.AppNotice.Emit(win, contract.AppNoticeEvent{Text: imported.Reason, Level: "warn"})
	}

	createTray(app, win)
	app.StartActiveWatch()
}

func applyTheme(mode string) {
	switch mode {
	case "light":
		mygo.Theme.SetSource(mygo.ThemeLight)
	case "dark":
		mygo.Theme.SetSource(mygo.ThemeDark)
	default:
		mygo.Theme.SetSource(mygo.ThemeSystem)
	}
}

func installTitleBarEvents(win *mygo.Window) {
	emit := func() {
		_ = service.WindowState.Emit(win, contract.WindowStateEvent{
			Maximized: win.IsMaximized(),
			Focused:   win.IsFocused(),
		})
	}
	win.OnFocus(emit)
	win.OnBlur(emit)
	win.OnMaximize(emit)
	win.OnUnmaximize(emit)
	emit()
}

func watchTheme(app *service.App, win *mygo.Window) {
	mygo.Theme.OnUpdated(func() {
		resolved := "light"
		if mygo.Theme.IsDark() {
			resolved = "dark"
		}
		_ = service.ThemeChanged.Emit(win, map[string]string{
			"mode":     app.Settings.Get().Theme,
			"resolved": resolved,
		})
	})
}

func installCloseGuard(app *service.App, win *mygo.Window) {
	win.OnClose(func(e *mygo.CloseEvent) {
		if app.IsQuitting() {
			return
		}
		e.PreventDefault()
		go func() {
			if app.IsDirty() {
				res, _ := mygo.Dialog.Message(mygo.MessageOptions{
					Parent:  win,
					Type:    mygo.MessageQuestion,
					Message: "有未保存的修改",
					Detail:  "丢弃后这些修改不会写回磁盘。",
					Buttons: []string{"继续编辑", "丢弃并关闭"},
				})
				if res.Button != 1 {
					return
				}
			}
			behavior := app.Settings.Get().CloseBehavior
			askResult := ""
			if behavior == "ask" {
				res, _ := mygo.Dialog.Message(mygo.MessageOptions{
					Parent:        win,
					Type:          mygo.MessageQuestion,
					Message:       "关闭窗口",
					Detail:        "挂到托盘后应用继续在后台运行。",
					Buttons:       []string{"挂到托盘", "退出应用"},
					CheckboxLabel: "记住我的选择",
				})
				if res.Button == 1 {
					askResult = "quit"
				} else {
					askResult = "tray"
				}
				if res.CheckboxChecked {
					remembered := "tray"
					if askResult == "quit" {
						remembered = "quit"
					}
					_, _ = app.Settings.Patch(contract.SettingsPatch{CloseBehavior: &remembered})
				}
			}
			switch service.DecideClose(behavior, false, app.TrayOK(), "discard", askResult) {
			case service.CloseQuit:
				app.MarkQuitting()
				mygo.App.Quit()
			case service.CloseHide:
				win.Hide()
			}
		}()
	})
}

func createTray(app *service.App, win *mygo.Window) {
	tray, err := mygo.NewTray(mygo.TrayOptions{
		Icon:    trayPNG,
		ToolTip: windowTitle,
		Menu: mygo.NewMenu([]*mygo.MenuItem{
			{Label: "显示主窗口", Click: func(*mygo.MenuItem, *mygo.Window) { app.ShowWindow() }},
			mygo.Separator(),
			{Label: "退出", Click: func(*mygo.MenuItem, *mygo.Window) {
				app.MarkQuitting()
				mygo.App.Quit()
			}},
		}),
	})
	if err != nil {
		// Without a tray the window must never hide: it could not come back.
		app.SetTray(nil, false)
		log.Println("tray:", err)
		return
	}
	tray.OnClick(app.ShowWindow)
	app.SetTray(tray, true)
}
