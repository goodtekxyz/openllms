package httpserver

import (
	"io/fs"
	"net/http"
	"strings"
)

const (
	siteHeaderPartial = "static/partials/site-header.html"
	siteFooterPartial = "static/partials/site-footer.html"

	// Markers — one chrome concept; aliases kept for existing HTML bodies.
	markerSiteHeader = "<!--LLMS_SITE_HEADER-->"
	markerSiteFooter = "<!--LLMS_SITE_FOOTER-->"
	markerPubHeader  = "<!--LLMS_PUBLIC_HEADER-->"
	markerPubFooter  = "<!--LLMS_PUBLIC_FOOTER-->"
	markerAppHeader  = "<!--LLMS_APP_HEADER-->"
	markerAppFooter  = "<!--LLMS_APP_FOOTER-->"
	markerAdminHeader = "<!--LLMS_ADMIN_HEADER-->"
)

type siteChromeLocale struct {
	HomeLabel, APILabel, InstallLabel, SelfHostLabel, ConsoleLabel string
	APIDocsLabel                                                   string
	HomeHref, APIHref, InstallHref, SelfHostHref                   string
	NavAria                                                        string
	ThemeSystem, ThemeLight, ThemeDark                             string
	LangSummary, LangAria                                          string
	Tagline                                                        string
	ProductAria, ProductTitle                                      string
	AppsAria, AppsTitle                                            string
	LegalAria, LegalTitle                                          string
	GTHome, GTAbout, GTContact                                     string
	PrivacyLabel, DisclaimerLabel                                  string
	LogoLabel                                                      string
	LogoHref                                                       string
}

func siteChromeLocaleFor(lang string) siteChromeLocale {
	switch lang {
	case "en":
		return siteChromeLocale{
			HomeLabel: "Home", APILabel: "API", InstallLabel: "Get started",
			SelfHostLabel: "Self-host", ConsoleLabel: "Console", APIDocsLabel: "API docs",
			HomeHref: "/en", APIHref: "/en#api", InstallHref: "/en/install", SelfHostHref: "/en#self-host",
			NavAria: "Main", ThemeSystem: "Theme: System", ThemeLight: "Theme: Light",
			ThemeDark: "Theme: Dark", LangSummary: "English", LangAria: "Language",
			Tagline: "Claude & Codex subscriptions and API keys — one URL.",
			ProductAria: "Product", ProductTitle: "Product",
			AppsAria: "Apps", AppsTitle: "Apps",
			LegalAria: "Legal", LegalTitle: "Legal",
			GTHome: "Home", GTAbout: "About", GTContact: "Contact",
			PrivacyLabel: "Privacy", DisclaimerLabel: "Disclaimer",
			LogoLabel: "llms", LogoHref: "/en",
		}
	case "ja":
		return siteChromeLocale{
			HomeLabel: "ホーム", APILabel: "API", InstallLabel: "はじめる",
			SelfHostLabel: "セルフホスト", ConsoleLabel: "コンソール", APIDocsLabel: "API ドキュメント",
			HomeHref: "/ja", APIHref: "/ja#api", InstallHref: "/ja/install", SelfHostHref: "/ja#self-host",
			NavAria: "メイン", ThemeSystem: "テーマ: システム", ThemeLight: "テーマ: ライト",
			ThemeDark: "テーマ: ダーク", LangSummary: "日本語", LangAria: "言語",
			Tagline: "Claude・Codex のサブスクと API キーを、URL ひとつに。",
			ProductAria: "製品", ProductTitle: "製品",
			AppsAria: "アプリ", AppsTitle: "アプリ",
			LegalAria: "法的情報", LegalTitle: "法的情報",
			GTHome: "ホーム", GTAbout: "About", GTContact: "お問い合わせ",
			PrivacyLabel: "プライバシー", DisclaimerLabel: "免責事項",
			LogoLabel: "llms", LogoHref: "/ja",
		}
	case "zh":
		return siteChromeLocale{
			HomeLabel: "首页", APILabel: "API", InstallLabel: "开始使用",
			SelfHostLabel: "自托管", ConsoleLabel: "控制台", APIDocsLabel: "API 文档",
			HomeHref: "/zh", APIHref: "/zh#api", InstallHref: "/zh/install", SelfHostHref: "/zh#self-host",
			NavAria: "主导航", ThemeSystem: "主题: 系统", ThemeLight: "主题: 浅色",
			ThemeDark: "主题: 深色", LangSummary: "中文", LangAria: "语言",
			Tagline: "Claude、Codex 订阅与 API 密钥 — 一个地址。",
			ProductAria: "产品", ProductTitle: "产品",
			AppsAria: "应用", AppsTitle: "应用",
			LegalAria: "法律", LegalTitle: "法律",
			GTHome: "首页", GTAbout: "关于", GTContact: "联系",
			PrivacyLabel: "隐私政策", DisclaimerLabel: "免责声明",
			LogoLabel: "llms", LogoHref: "/zh",
		}
	default:
		return siteChromeLocale{
			HomeLabel: "홈", APILabel: "API", InstallLabel: "시작하기",
			SelfHostLabel: "셀프호스트", ConsoleLabel: "콘솔", APIDocsLabel: "API 문서",
			HomeHref: "/", APIHref: "/#api", InstallHref: "/ko/install", SelfHostHref: "/#self-host",
			NavAria: "메인", ThemeSystem: "테마: 시스템", ThemeLight: "테마: 라이트",
			ThemeDark: "테마: 다크", LangSummary: "한국어", LangAria: "언어",
			Tagline: "Claude·Codex 구독과 API 키를, 주소 하나로.",
			ProductAria: "제품", ProductTitle: "제품",
			AppsAria: "앱", AppsTitle: "앱",
			LegalAria: "법적 고지", LegalTitle: "법적 고지",
			GTHome: "홈", GTAbout: "소개", GTContact: "문의",
			PrivacyLabel: "개인정보처리방침", DisclaimerLabel: "면책 조항",
			LogoLabel: "llms", LogoHref: "/",
		}
	}
}

func serveChromeHTML(w http.ResponseWriter, r *http.Request, bodyFile, activeNav, lang string) {
	body, err := fs.ReadFile(publicStatic, bodyFile)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	headerTpl, err := fs.ReadFile(publicStatic, siteHeaderPartial)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	footerTpl, err := fs.ReadFile(publicStatic, siteFooterPartial)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	loc := siteChromeLocaleFor(lang)
	if activeNav == "admin" {
		loc.LogoLabel = "llms admin"
		loc.LogoHref = "/admin"
	}
	header := buildSiteHeader(string(headerTpl), activeNav, lang, loc)
	footer := buildSiteFooter(string(footerTpl), loc)

	out := string(body)
	out = replaceFirstAny(out, header, markerSiteHeader, markerPubHeader, markerAppHeader, markerAdminHeader)
	out = replaceFirstAny(out, footer, markerSiteFooter, markerPubFooter, markerAppFooter)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Disposition", "inline")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = w.Write([]byte(out))
}

func replaceFirstAny(haystack, replacement string, markers ...string) string {
	for _, m := range markers {
		if strings.Contains(haystack, m) {
			return strings.Replace(haystack, m, replacement, 1)
		}
	}
	return haystack
}

func buildSiteHeader(header, activeNav, lang string, loc siteChromeLocale) string {
	activeAttr := "aria-current=\"page\""
	h := strings.ReplaceAll(header, "{{ACTIVE_HOME}}", attrIf(activeNav == "home", activeAttr))
	h = strings.ReplaceAll(h, "{{ACTIVE_API}}", attrIf(activeNav == "api", activeAttr))
	h = strings.ReplaceAll(h, "{{ACTIVE_INSTALL}}", attrIf(activeNav == "install", activeAttr))
	h = strings.ReplaceAll(h, "{{ACTIVE_SELFHOST}}", attrIf(activeNav == "selfhost", activeAttr))
	h = strings.ReplaceAll(h, "{{ACTIVE_CONSOLE}}", attrIf(activeNav == "console", activeAttr))
	h = strings.ReplaceAll(h, "{{HOME_LABEL}}", loc.HomeLabel)
	h = strings.ReplaceAll(h, "{{API_LABEL}}", loc.APILabel)
	h = strings.ReplaceAll(h, "{{INSTALL_LABEL}}", loc.InstallLabel)
	h = strings.ReplaceAll(h, "{{SELFHOST_LABEL}}", loc.SelfHostLabel)
	h = strings.ReplaceAll(h, "{{CONSOLE_LABEL}}", loc.ConsoleLabel)
	h = strings.ReplaceAll(h, "{{HOME_HREF}}", loc.HomeHref)
	h = strings.ReplaceAll(h, "{{API_HREF}}", loc.APIHref)
	h = strings.ReplaceAll(h, "{{INSTALL_HREF}}", loc.InstallHref)
	h = strings.ReplaceAll(h, "{{SELFHOST_HREF}}", loc.SelfHostHref)
	h = strings.ReplaceAll(h, "{{LOGO_HREF}}", loc.LogoHref)
	h = strings.ReplaceAll(h, "{{LOGO_LABEL}}", loc.LogoLabel)
	h = strings.ReplaceAll(h, "{{NAV_ARIA}}", loc.NavAria)
	h = strings.ReplaceAll(h, "{{THEME_SYSTEM}}", loc.ThemeSystem)
	h = strings.ReplaceAll(h, "{{THEME_LIGHT}}", loc.ThemeLight)
	h = strings.ReplaceAll(h, "{{THEME_DARK}}", loc.ThemeDark)
	h = strings.ReplaceAll(h, "{{LANG_SUMMARY}}", loc.LangSummary)
	h = strings.ReplaceAll(h, "{{LANG_ARIA}}", loc.LangAria)

	page := "landing"
	if activeNav == "install" {
		page = "install"
	}
	h = strings.ReplaceAll(h, "{{LANG_KO_HREF}}", publicLangHref("ko", page))
	h = strings.ReplaceAll(h, "{{LANG_EN_HREF}}", publicLangHref("en", page))
	h = strings.ReplaceAll(h, "{{LANG_JA_HREF}}", publicLangHref("ja", page))
	h = strings.ReplaceAll(h, "{{LANG_ZH_HREF}}", publicLangHref("zh", page))
	langCurrent := "aria-current=\"true\""
	h = strings.ReplaceAll(h, "{{LANG_KO_CURRENT}}", attrIf(lang == "ko", langCurrent))
	h = strings.ReplaceAll(h, "{{LANG_EN_CURRENT}}", attrIf(lang == "en", langCurrent))
	h = strings.ReplaceAll(h, "{{LANG_JA_CURRENT}}", attrIf(lang == "ja", langCurrent))
	h = strings.ReplaceAll(h, "{{LANG_ZH_CURRENT}}", attrIf(lang == "zh", langCurrent))
	return h
}

func buildSiteFooter(footer string, loc siteChromeLocale) string {
	f := footer
	repl := map[string]string{
		"{{HOME_HREF}}": loc.HomeHref, "{{API_HREF}}": loc.APIHref, "{{INSTALL_HREF}}": loc.InstallHref,
		"{{SELFHOST_HREF}}": loc.SelfHostHref,
		"{{HOME_LABEL}}": loc.HomeLabel, "{{API_LABEL}}": loc.APILabel, "{{INSTALL_LABEL}}": loc.InstallLabel,
		"{{SELFHOST_LABEL}}": loc.SelfHostLabel, "{{CONSOLE_LABEL}}": loc.ConsoleLabel,
		"{{API_DOCS_LABEL}}": loc.APIDocsLabel, "{{TAGLINE}}": loc.Tagline,
		"{{PRODUCT_ARIA}}": loc.ProductAria, "{{PRODUCT_TITLE}}": loc.ProductTitle,
		"{{APPS_ARIA}}": loc.AppsAria, "{{APPS_TITLE}}": loc.AppsTitle,
		"{{LEGAL_ARIA}}": loc.LegalAria, "{{LEGAL_TITLE}}": loc.LegalTitle,
		"{{GT_HOME}}": loc.GTHome, "{{GT_ABOUT}}": loc.GTAbout, "{{GT_CONTACT}}": loc.GTContact,
		"{{PRIVACY_LABEL}}": loc.PrivacyLabel, "{{DISCLAIMER_LABEL}}": loc.DisclaimerLabel,
	}
	for k, v := range repl {
		f = strings.ReplaceAll(f, k, v)
	}
	return f
}

// publicLangHref returns language-switch targets. Korean always goes through
// /ko[…] so the lang cookie is rewritten before the canonical / or /install page.
func publicLangHref(lang, page string) string {
	if page == "install" {
		if lang == "ko" {
			return "/ko/install"
		}
		return installPathForLang(lang)
	}
	if lang == "ko" {
		return "/ko"
	}
	return landingPathForLang(lang)
}

func attrIf(ok bool, attr string) string {
	if ok {
		return attr
	}
	return ""
}

// Compatibility wrappers — same injector.
func servePublicHTML(w http.ResponseWriter, r *http.Request, bodyFile, activeNav, lang string) {
	serveChromeHTML(w, r, bodyFile, activeNav, lang)
}

func serveAppPage(w http.ResponseWriter, r *http.Request, bodyFile, activeNav, locale string) {
	serveChromeHTML(w, r, bodyFile, activeNav, locale)
}

func serveAdminPage(w http.ResponseWriter, r *http.Request) {
	serveChromeHTML(w, r, "static/admin.html", "admin", "ko")
}
