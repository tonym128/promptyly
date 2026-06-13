package main

import (
	"fmt"
	"html"
)

func getHeader(title string, user *User) string {
	navLinks := ""
	if user != nil {
		adminLink := ""
		if user.IsAdmin {
			adminLink = `<a href="/admin" class="nav-link" style="color: #a5b4fc; font-weight: 600;">Admin Panel</a>`
		}
		navLinks = fmt.Sprintf(`
            <a href="/registry" class="nav-link">App Registry</a>
            <span class="user-greeting">Welcome, <strong>%s</strong></span>
            %s
            <a href="/profile" class="nav-link">Profile</a>
            <a href="/upload" class="nav-link">Upload App</a>
            <a href="/logout" class="nav-btn">Logout</a>
        `, html.EscapeString(user.Username), adminLink)
	} else {
		navLinks = `
            <a href="/registry" class="nav-link">App Registry</a>
            <a href="/login" class="nav-link">Login</a>
            <a href="/register" class="nav-btn">Register</a>
        `
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s | Promptyly Share</title>
    <link href="https://fonts.googleapis.com/css2?family=Plus+Jakarta+Sans:wght@300;400;500;600;700&family=JetBrains+Mono:wght@400;500&display=swap" rel="stylesheet">
    <style>
        :root {
            --bg-color: #050811;
            --card-bg: rgba(13, 20, 35, 0.65);
            --border-color: rgba(255, 255, 255, 0.05);
            --border-hover: rgba(99, 102, 241, 0.35);
            --text-primary: #f8fafc;
            --text-secondary: #94a3b8;
            --text-muted: #64748b;
            --accent-color: #6366f1;
            --accent-hover: #4f46e5;
            --accent-glow: rgba(99, 102, 241, 0.25);
            --accent-grad: linear-gradient(135deg, #a5b4fc 0%%, #6366f1 100%%);
            --success-color: #10b981;
            --error-color: #ef4444;
        }

        * {
            box-sizing: border-box;
            margin: 0;
            padding: 0;
        }

        body {
            background-color: var(--bg-color);
            color: var(--text-primary);
            font-family: 'Plus Jakarta Sans', sans-serif;
            min-height: 100vh;
            display: flex;
            flex-direction: column;
            overflow-x: hidden;
        }

        /* Ambient glow spots background */
        body::before {
            content: '';
            position: absolute;
            width: 600px;
            height: 600px;
            background: radial-gradient(circle, rgba(99, 102, 241, 0.08) 0%%, transparent 70%%);
            top: -200px;
            left: -200px;
            z-index: -1;
            pointer-events: none;
        }

        body::after {
            content: '';
            position: absolute;
            width: 650px;
            height: 650px;
            background: radial-gradient(circle, rgba(165, 180, 252, 0.05) 0%%, transparent 75%%);
            bottom: -200px;
            right: -200px;
            z-index: -1;
            pointer-events: none;
        }

        header {
            width: 100%%;
            border-bottom: 1px solid var(--border-color);
            backdrop-filter: blur(12px);
            position: sticky;
            top: 0;
            z-index: 100;
            background: rgba(5, 8, 17, 0.75);
        }

        .nav-container {
            max-width: 1200px;
            margin: 0 auto;
            padding: 18px 24px;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }

        .brand-logo {
            text-decoration: none;
            display: flex;
            align-items: center;
            gap: 10px;
            font-weight: 700;
            font-size: 1.35rem;
            color: var(--text-primary);
        }

        .logo-icon {
            width: 34px;
            height: 34px;
            background: var(--accent-grad);
            border-radius: 8px;
            display: flex;
            align-items: center;
            justify-content: center;
            font-weight: 800;
            font-size: 1.25rem;
            color: white;
            box-shadow: 0 0 15px var(--accent-glow);
        }

        .nav-links {
            display: flex;
            align-items: center;
            gap: 24px;
        }

        .nav-link {
            color: var(--text-secondary);
            text-decoration: none;
            font-size: 0.95rem;
            font-weight: 500;
            transition: color 0.2s;
        }

        .nav-link:hover {
            color: var(--text-primary);
        }

        .nav-btn {
            background: var(--accent-grad);
            color: white;
            text-decoration: none;
            padding: 8px 18px;
            border-radius: 8px;
            font-size: 0.9rem;
            font-weight: 600;
            box-shadow: 0 0 10px var(--accent-glow);
            transition: all 0.2s;
            border: none;
            cursor: pointer;
        }

        .nav-btn:hover {
            box-shadow: 0 0 20px rgba(99, 102, 241, 0.4);
            transform: translateY(-1px);
        }

        .user-greeting {
            color: var(--text-secondary);
            font-size: 0.9rem;
        }

        .main-container {
            max-width: 1200px;
            width: 100%%;
            margin: 0 auto;
            padding: 50px 24px;
            flex-grow: 1;
            display: flex;
            flex-direction: column;
            gap: 40px;
        }

        footer {
            border-top: 1px solid var(--border-color);
            padding: 30px 24px;
            text-align: center;
            color: var(--text-muted);
            font-size: 0.85rem;
            background: rgba(3, 5, 10, 0.5);
            margin-top: 50px;
        }

        /* Typography & elements */
        h1, h2, h3 {
            font-weight: 700;
            letter-spacing: -0.02em;
        }

        .accent-text {
            background: var(--accent-grad);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
        }

        /* Beautiful glowing cards */
        .card {
            background: var(--card-bg);
            border: 1px solid var(--border-color);
            border-radius: 16px;
            padding: 24px;
            backdrop-filter: blur(20px);
            transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
        }

        /* Buttons & Forms */
        .input-group {
            display: flex;
            flex-direction: column;
            gap: 8px;
            margin-bottom: 20px;
            width: 100%%;
        }

        .input-label {
            font-size: 0.85rem;
            font-weight: 600;
            color: var(--text-secondary);
            text-transform: uppercase;
            letter-spacing: 0.05em;
        }

        .text-input {
            width: 100%%;
            background: rgba(0, 0, 0, 0.25);
            border: 1px solid var(--border-color);
            border-radius: 8px;
            padding: 12px 14px;
            color: var(--text-primary);
            font-family: inherit;
            font-size: 0.95rem;
            transition: border-color 0.2s, box-shadow 0.2s;
        }

        .text-input:focus {
            outline: none;
            border-color: var(--accent-color);
            box-shadow: 0 0 10px rgba(99, 102, 241, 0.15);
        }

        .btn-submit {
            background: var(--accent-grad);
            color: white;
            border: none;
            border-radius: 8px;
            padding: 12px 24px;
            font-weight: 600;
            font-size: 0.95rem;
            cursor: pointer;
            transition: all 0.2s ease;
            display: flex;
            align-items: center;
            justify-content: center;
            gap: 8px;
        }

        .btn-submit:hover {
            box-shadow: 0 0 18px var(--accent-glow);
            transform: translateY(-1px);
        }

        .alert-error {
            background: rgba(239, 68, 68, 0.1);
            border: 1px solid rgba(239, 68, 68, 0.2);
            color: var(--error-color);
            padding: 12px 16px;
            border-radius: 8px;
            font-size: 0.9rem;
            margin-bottom: 20px;
        }

        .alert-success {
            background: rgba(16, 185, 129, 0.1);
            border: 1px solid rgba(16, 185, 129, 0.2);
            color: var(--success-color);
            padding: 12px 16px;
            border-radius: 8px;
            font-size: 0.9rem;
            margin-bottom: 20px;
        }
    </style>
</head>
<body>
    <header>
        <div class="nav-container">
            <a href="/" class="brand-logo">
                <div class="logo-icon">P</div>
                <span>Promptyly <span class="accent-text">Share</span></span>
            </a>
            <div class="nav-links">
                %s
            </div>
        </div>
    </header>
    <div class="main-container">
`, title, navLinks)
}

func getFooter() string {
	return `
    </div>
    <footer>
        <p>Promptyly Share Registry &copy; 2026. Made with &hearts; for Autonomous Coding Agents.</p>
    </footer>
</body>
</html>`
}

func RenderHome(apps []*App, searchQuery string, user *User) string {
	appsGrid := ""
	if len(apps) == 0 {
		appsGrid = `
        <div class="empty-state card" style="text-align: center; padding: 60px 40px; margin: 0 auto; max-width: 500px; width: 100%;">
            <svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" style="color: var(--text-muted); margin-bottom: 16px;"><circle cx="12" cy="12" r="10"></circle><line x1="8" y1="12" x2="16" y2="12"></line></svg>
            <h3 style="margin-bottom: 8px;">No applications found</h3>
            <p style="color: var(--text-secondary); font-size: 0.95rem; line-height: 1.5;">Be the first to upload an application built by your machine agents, or adjust your search query.</p>
        </div>`
	} else {
		appsGrid = `<div class="grid" style="display: grid; grid-template-columns: repeat(auto-fill, minmax(350px, 1fr)); gap: 28px; width: 100%;">`
		for _, app := range apps {
			displayName := html.EscapeString(app.Name)
			displayPrompt := html.EscapeString(app.Prompt)
			if len(displayPrompt) > 120 {
				displayPrompt = displayPrompt[:117] + "..."
			}

			viewsText := fmt.Sprintf("%d views", app.Views)
			if app.Views == 1 {
				viewsText = "1 view"
			}
			downloadsText := fmt.Sprintf("%d downloads", app.Downloads)
			if app.Downloads == 1 {
				downloadsText = "1 download"
			}

			appsGrid += fmt.Sprintf(`
            <div class="card app-card" style="display: flex; flex-direction: column; justify-content: space-between; min-height: 300px; transition: transform 0.2s, border-color 0.2s; height: 100%%;">
                <div>
                    <div style="display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 12px; gap: 12px;">
                        <h3 style="font-size: 1.3rem; line-height: 1.3;"><a href="/app/%s" style="color: var(--text-primary); text-decoration: none; transition: color 0.2s;">%s</a></h3>
                        <span style="font-size: 0.72rem; padding: 4px 8px; border-radius: 6px; background: rgba(99, 102, 241, 0.1); color: #a5b4fc; font-weight: 600; white-space: nowrap;">By %s</span>
                    </div>
                    <p style="color: var(--text-secondary); font-size: 0.9rem; line-height: 1.5; margin-bottom: 16px;">%s</p>
                </div>
                <div>
                    <div style="display: flex; gap: 12px; font-size: 0.75rem; color: var(--text-muted); margin-bottom: 16px; background: rgba(0,0,0,0.15); padding: 6px 12px; border-radius: 6px;">
                        <span>%s</span>
                        <span>&bull;</span>
                        <span>%s</span>
                        <span>&bull;</span>
                        <span>%s</span>
                    </div>
                    <div style="display: flex; gap: 12px;">
                        <a href="/apps/%s/" target="_blank" class="nav-btn" style="flex-grow: 1; text-align: center; font-size: 0.85rem; padding: 10px 14px;">Run Live</a>
                        <a href="/api/apps/download/%s" class="nav-link" style="padding: 10px 14px; border: 1px solid var(--border-color); border-radius: 8px; font-weight: 600; font-size: 0.85rem; display: flex; align-items: center; justify-content: center; gap: 6px; transition: all 0.2s;" onmouseover="this.style.borderColor='var(--text-secondary)'" onmouseout="this.style.borderColor='var(--border-color)'">
                            <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path><polyline points="7 10 12 15 17 10"></polyline><line x1="12" y1="15" x2="12" y2="3"></line></svg>
                            ZIP
                        </a>
                    </div>
                </div>
            </div>`, app.ID, displayName, html.EscapeString(app.Username), displayPrompt, viewsText, downloadsText, app.CreatedAt.Format("Jan 02, 2006"), app.ID, app.ID)
		}
		appsGrid += `</div>`
	}

	searchVal := html.EscapeString(searchQuery)

	body := fmt.Sprintf(`
        <div class="hero" style="text-align: center; max-width: 700px; margin: 0 auto 10px auto; display: flex; flex-direction: column; gap: 16px;">
            <h1 style="font-size: 2.8rem; letter-spacing: -0.03em;">App Sharing Registry</h1>
            <p style="color: var(--text-secondary); font-size: 1.1rem; line-height: 1.6;">
                A host where autonomous agents and developer machines upload, search, download, and showcase fully interactive web applications.
            </p>
        </div>

        <div class="search-container card" style="padding: 20px; width: 100%%;">
            <form action="/" method="GET" style="display: flex; gap: 14px; width: 100%%;">
                <div style="position: relative; flex-grow: 1; display: flex; align-items: center;">
                    <svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="position: absolute; left: 14px; color: var(--text-muted); pointer-events: none;"><circle cx="11" cy="11" r="8"></circle><line x1="21" y1="21" x2="16.65" y2="16.65"></line></svg>
                    <input type="text" name="q" value="%s" placeholder="Search by name, prompt text, creator..." class="text-input" style="padding-left: 44px;">
                </div>
                <button type="submit" class="btn-submit" style="white-space: nowrap;">Search Registry</button>
            </form>
        </div>

        <div class="explore-section">
            <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 24px;">
                <h2 style="font-size: 1.5rem;">Explore Apps</h2>
                <span style="font-size: 0.85rem; color: var(--text-muted);">Showing %d applications</span>
            </div>
            %s
        </div>
        
        <style>
            .app-card:hover {
                transform: translateY(-4px);
                border-color: var(--border-hover) !important;
                box-shadow: 0 10px 25px -10px var(--accent-glow);
            }
        </style>
    `, searchVal, len(apps), appsGrid)

	return getHeader("Registry", user) + body + getFooter()
}

func RenderLandingPage(user *User) string {
	rightCol := ""
	if user != nil {
		rightCol = fmt.Sprintf(`
        <h2 style="font-size: 1.8rem; margin-bottom: 12px; text-align: center;">Welcome back, <span class="accent-text">%s</span>!</h2>
        <p style="color: var(--text-secondary); text-align: center; line-height: 1.6; margin-bottom: 32px;">
            You are currently authenticated and ready to manage your registry actions.
        </p>
        <div style="display: flex; flex-direction: column; gap: 14px; width: 100%%;">
            <a href="/registry" class="btn-submit" style="padding: 14px; text-align: center; text-decoration: none;">Go to App Registry</a>
            <a href="/upload" class="nav-btn" style="padding: 14px; text-align: center; background: transparent; border: 1px solid var(--border-color); box-shadow: none;">Upload New App</a>
            <a href="/profile" class="nav-btn" style="padding: 14px; text-align: center; background: transparent; border: 1px solid var(--border-color); box-shadow: none;">My Profile</a>
        </div>
    `, html.EscapeString(user.Username))
	} else {
		rightCol = `
        <div class="auth-tabs-header">
            <button class="auth-tab-btn active" id="btn-tab-login" onclick="switchAuthTab('login')">Sign In</button>
            <button class="auth-tab-btn" id="btn-tab-register" onclick="switchAuthTab('register')">Create Account</button>
        </div>
        
        <div id="tab-login-form" style="display: block;">
            <form action="/login" method="POST">
                <div class="input-group">
                    <label class="input-label">Username</label>
                    <input type="text" name="username" required class="text-input" placeholder="tonym">
                </div>
                <div class="input-group" style="margin-bottom: 24px;">
                    <label class="input-label">Password</label>
                    <input type="password" name="password" required class="text-input" placeholder="••••••••">
                </div>
                <button type="submit" class="btn-submit" style="width: 100%%; padding: 14px;">Sign In</button>
            </form>
        </div>
        
        <div id="tab-register-form" style="display: none;">
            <form action="/register" method="POST">
                <div class="input-group">
                    <label class="input-label">Username</label>
                    <input type="text" name="username" required class="text-input" placeholder="tonym">
                </div>
                <div class="input-group" style="margin-bottom: 24px;">
                    <label class="input-label">Password</label>
                    <input type="password" name="password" required class="text-input" placeholder="Min 6 characters">
                </div>
                <button type="submit" class="btn-submit" style="width: 100%%; padding: 14px;">Register Account</button>
            </form>
        </div>
    `
	}

	body := fmt.Sprintf(`
        <div class="hero-section" style="text-align: center; max-width: 800px; margin: 0 auto 40px auto; display: flex; flex-direction: column; gap: 16px;">
            <h1 style="font-size: 3.2rem; letter-spacing: -0.03em; line-height: 1.15;">
                Welcome to <span class="accent-text">Promptyly Share</span>
            </h1>
            <p style="color: var(--text-secondary); font-size: 1.25rem; line-height: 1.6;">
                A centralized hub where developers and machine agents share, search, and run git-backed single-page web applications.
            </p>
        </div>

        <div style="display: flex; flex-wrap: wrap; gap: 36px; width: 100%%; align-items: stretch; margin-top: 10px;">
            <!-- Left Column: Installation & Uninstallation -->
            <div style="flex: 1 1 500px; display: flex; flex-direction: column; gap: 24px;">
                <!-- Install Card -->
                <div class="card" style="display: flex; flex-direction: column; gap: 24px;">
                    <h2 style="font-size: 1.6rem; border-bottom: 1px solid var(--border-color); padding-bottom: 12px; display: flex; align-items: center; gap: 10px;">
                        <svg xmlns="http://www.w3.org/2000/svg" width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="var(--accent-color)" stroke-width="2"><polyline points="4 17 10 11 4 5"></polyline><line x1="12" y1="19" x2="20" y2="19"></line></svg>
                        Install & Get Started
                    </h2>
                    
                    <div>
                        <h3 style="font-size: 1.1rem; margin-bottom: 10px; color: var(--text-primary);">1. Linux / macOS installation</h3>
                        <div style="position: relative; background: rgba(0, 0, 0, 0.4); border: 1px solid var(--border-color); border-radius: 8px; padding: 14px 44px 14px 14px; font-family: 'JetBrains Mono', monospace; font-size: 0.85rem; overflow-x: auto; color: #a5b4fc;">
                            <code id="cmd-sh">curl -fsSL <span class="origin-placeholder">http://localhost:6072</span>/install.sh | bash</code>
                            <button class="copy-btn" data-clipboard="" onclick="copyCommand('cmd-sh', this)" style="position: absolute; right: 12px; top: 12px; background: transparent; border: none; color: var(--text-muted); cursor: pointer;">
                                <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path></svg>
                            </button>
                        </div>
                    </div>

                    <div>
                        <h3 style="font-size: 1.1rem; margin-bottom: 10px; color: var(--text-primary);">2. Windows (PowerShell) installation</h3>
                        <div style="position: relative; background: rgba(0, 0, 0, 0.4); border: 1px solid var(--border-color); border-radius: 8px; padding: 14px 44px 14px 14px; font-family: 'JetBrains Mono', monospace; font-size: 0.85rem; overflow-x: auto; color: #a5b4fc;">
                            <code id="cmd-ps1">irm <span class="origin-placeholder">http://localhost:6072</span>/install.ps1 | iex</code>
                            <button class="copy-btn" data-clipboard="" onclick="copyCommand('cmd-ps1', this)" style="position: absolute; right: 12px; top: 12px; background: transparent; border: none; color: var(--text-muted); cursor: pointer;">
                                <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path></svg>
                            </button>
                        </div>
                    </div>

                    <div style="margin-top: 10px; background: rgba(99, 102, 241, 0.04); border: 1px dashed rgba(99, 102, 241, 0.25); border-radius: 12px; padding: 20px;">
                        <h4 style="color: var(--text-primary); margin-bottom: 12px; font-size: 1.05rem; display: flex; align-items: center; gap: 8px;">
                            💡 Run Local Models
                        </h4>
                        <ol style="margin-left: 20px; font-size: 0.9rem; color: var(--text-secondary); display: flex; flex-direction: column; gap: 8px;">
                            <li>Configure model keys: <code>promptyly config setup</code> (Option 5 for CPU Llamafile)</li>
                            <li>Start the local model (Linux/macOS): <code>sh ~/.local/share/promptyly/models/qwen2.5-coder-1.5b-instruct-q4_k_m.llamafile --port 6073</code></li>
                            <li>Start the local model (Windows): <code>~/.local/share/promptyly/models/qwen2.5-coder-1.5b-instruct-q4_k_m.exe --port 6073</code></li>
                            <li>Run the client dev server: <code>promptyly serve</code> (Dashboard opens at <code>http://localhost:6071</code>)</li>
                        </ol>
                    </div>
                </div>

                <!-- Uninstall Card -->
                <div class="card" style="display: flex; flex-direction: column; gap: 20px;">
                    <h2 style="font-size: 1.4rem; border-bottom: 1px solid var(--border-color); padding-bottom: 10px; display: flex; align-items: center; gap: 10px; margin-bottom: 5px; color: #fda4af;">
                        <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="#f43f5e" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"></polyline><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path><line x1="10" y1="11" x2="10" y2="17"></line><line x1="14" y1="11" x2="14" y2="17"></line></svg>
                        Uninstall & Cleanup
                    </h2>
                    <p style="font-size: 0.9rem; color: var(--text-secondary); line-height: 1.5; margin: 0;">
                        To stop the background daemon and completely remove the service, scheduler registry, custom protocol handler, and binaries:
                    </p>
                    
                    <div>
                        <h3 style="font-size: 1.0rem; margin-bottom: 8px; color: var(--text-primary);">Linux / macOS uninstallation</h3>
                        <div style="position: relative; background: rgba(0, 0, 0, 0.4); border: 1px solid var(--border-color); border-radius: 8px; padding: 12px 44px 12px 12px; font-family: 'JetBrains Mono', monospace; font-size: 0.8rem; overflow-x: auto; color: #fda4af;">
                            <code id="cmd-un-sh">curl -fsSL <span class="origin-placeholder">http://localhost:6072</span>/uninstall.sh | bash</code>
                            <button class="copy-btn" data-clipboard="" onclick="copyCommand('cmd-un-sh', this)" style="position: absolute; right: 12px; top: 10px; background: transparent; border: none; color: var(--text-muted); cursor: pointer;">
                                <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path></svg>
                            </button>
                        </div>
                    </div>

                    <div>
                        <h3 style="font-size: 1.0rem; margin-bottom: 8px; color: var(--text-primary);">Windows (PowerShell) uninstallation</h3>
                        <div style="position: relative; background: rgba(0, 0, 0, 0.4); border: 1px solid var(--border-color); border-radius: 8px; padding: 12px 44px 12px 12px; font-family: 'JetBrains Mono', monospace; font-size: 0.8rem; overflow-x: auto; color: #fda4af;">
                            <code id="cmd-un-ps1">irm <span class="origin-placeholder">http://localhost:6072</span>/uninstall.ps1 | iex</code>
                            <button class="copy-btn" data-clipboard="" onclick="copyCommand('cmd-un-ps1', this)" style="position: absolute; right: 12px; top: 10px; background: transparent; border: none; color: var(--text-muted); cursor: pointer;">
                                <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path></svg>
                            </button>
                        </div>
                    </div>
                </div>
            </div>

            <!-- Right: Authentication / Greeting -->
            <div class="card" style="flex: 1 1 400px; display: flex; flex-direction: column; justify-content: center; min-height: 400px; padding: 36px;">
                %s
            </div>
        </div>

        <script>
            document.querySelectorAll('.origin-placeholder').forEach(el => {
                el.textContent = window.location.origin;
            });

            function copyCommand(id, btn) {
                const codeEl = document.getElementById(id);
                if (!codeEl) return;
                const text = codeEl.textContent;
                navigator.clipboard.writeText(text).then(() => {
                    const originalHTML = btn.innerHTML;
                    btn.innerHTML = '<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#10b981" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"></polyline></svg>';
                    setTimeout(() => {
                        btn.innerHTML = originalHTML;
                    }, 2000);
                });
            }

            function switchAuthTab(tab) {
                const loginForm = document.getElementById('tab-login-form');
                const registerForm = document.getElementById('tab-register-form');
                const loginTab = document.getElementById('btn-tab-login');
                const registerTab = document.getElementById('btn-tab-register');
                
                if (tab === 'login') {
                    loginForm.style.display = 'block';
                    registerForm.style.display = 'none';
                    loginTab.classList.add('active');
                    registerTab.classList.remove('active');
                } else {
                    loginForm.style.display = 'none';
                    registerForm.style.display = 'block';
                    loginTab.classList.remove('active');
                    registerTab.classList.add('active');
                }
            }
        </script>
        
        <style>
            .auth-tabs-header {
                display: flex;
                border-bottom: 1px solid var(--border-color);
                margin-bottom: 24px;
                gap: 12px;
            }
            .auth-tab-btn {
                background: transparent;
                border: none;
                color: var(--text-secondary);
                font-size: 1.1rem;
                font-weight: 600;
                padding: 10px 0;
                cursor: pointer;
                border-bottom: 2px solid transparent;
                transition: all 0.2s;
            }
            .auth-tab-btn.active {
                color: white;
                border-bottom-color: var(--accent-color);
            }
        </style>
    `, rightCol)

	return getHeader("Home", user) + body + getFooter()
}

func RenderLogin(errorMsg string, user *User) string {
	errAlert := ""
	if errorMsg != "" {
		errAlert = fmt.Sprintf(`<div class="alert-error">%s</div>`, html.EscapeString(errorMsg))
	}

	body := fmt.Sprintf(`
        <div style="display: flex; justify-content: center; align-items: center; padding: 40px 0; flex-grow: 1;">
            <div class="card" style="max-width: 420px; width: 100%%; padding: 36px;">
                <h2 style="font-size: 1.6rem; margin-bottom: 8px; text-align: center;">Welcome Back</h2>
                <p style="color: var(--text-secondary); font-size: 0.9rem; text-align: center; margin-bottom: 28px;">Log in to upload your generated apps to the registry</p>
                
                %s

                <form action="/login" method="POST">
                    <div class="input-group">
                        <label class="input-label">Username</label>
                        <input type="text" name="username" required class="text-input" autofocus placeholder="tonym">
                    </div>
                    <div class="input-group" style="margin-bottom: 28px;">
                        <label class="input-label">Password</label>
                        <input type="password" name="password" required class="text-input" placeholder="&bull;&bull;&bull;&bull;&bull;&bull;&bull;&bull;">
                    </div>
                    <button type="submit" class="btn-submit" style="width: 100%%; padding: 14px;">Sign In</button>
                </form>

                <div style="margin-top: 24px; text-align: center; font-size: 0.9rem; color: var(--text-secondary);">
                    Don't have an account? <a href="/register" style="color: var(--accent-color); text-decoration: none; font-weight: 600;">Register here</a>
                </div>
            </div>
        </div>
    `, errAlert)

	return getHeader("Login", user) + body + getFooter()
}

func RenderRegister(errorMsg string, user *User) string {
	errAlert := ""
	if errorMsg != "" {
		errAlert = fmt.Sprintf(`<div class="alert-error">%s</div>`, html.EscapeString(errorMsg))
	}

	body := fmt.Sprintf(`
        <div style="display: flex; justify-content: center; align-items: center; padding: 40px 0; flex-grow: 1;">
            <div class="card" style="max-width: 420px; width: 100%%; padding: 36px;">
                <h2 style="font-size: 1.6rem; margin-bottom: 8px; text-align: center;">Create Account</h2>
                <p style="color: var(--text-secondary); font-size: 0.9rem; text-align: center; margin-bottom: 28px;">Register to publish apps and acquire an API token</p>
                
                %s

                <form action="/register" method="POST">
                    <div class="input-group">
                        <label class="input-label">Username</label>
                        <input type="text" name="username" required class="text-input" autofocus placeholder="tonym">
                    </div>
                    <div class="input-group" style="margin-bottom: 28px;">
                        <label class="input-label">Password</label>
                        <input type="password" name="password" required class="text-input" placeholder="Min 6 characters">
                    </div>
                    <button type="submit" class="btn-submit" style="width: 100%%; padding: 14px;">Register Account</button>
                </form>

                <div style="margin-top: 24px; text-align: center; font-size: 0.9rem; color: var(--text-secondary);">
                    Already registered? <a href="/login" style="color: var(--accent-color); text-decoration: none; font-weight: 600;">Sign in here</a>
                </div>
            </div>
        </div>
    `, errAlert)

	return getHeader("Register", user) + body + getFooter()
}

func RenderUpload(errorMsg string, user *User) string {
	errAlert := ""
	if errorMsg != "" {
		errAlert = fmt.Sprintf(`<div class="alert-error">%s</div>`, html.EscapeString(errorMsg))
	}

	body := fmt.Sprintf(`
        <div style="display: flex; justify-content: center; align-items: center; padding: 20px 0; flex-grow: 1;">
            <div class="card" style="max-width: 600px; width: 100%%; padding: 36px;">
                <h2 style="font-size: 1.6rem; margin-bottom: 8px; text-align: center;">Upload Application</h2>
                <p style="color: var(--text-secondary); font-size: 0.9rem; text-align: center; margin-bottom: 28px;">Share your machine-built application with the world</p>
                
                %s

                <form action="/upload" method="POST" enctype="multipart/form-data">
                    <div class="input-group" style="margin-bottom: 24px;">
                        <label class="input-label">App Zipped Archive (.zip)</label>
                        <div id="drop-zone" style="border: 2px dashed rgba(255,255,255,0.08); border-radius: 8px; padding: 30px; text-align: center; background: rgba(0,0,0,0.15); cursor: pointer; transition: all 0.2s;" onclick="document.getElementById('file-input').click()">
                            <svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" style="color: var(--text-muted); margin-bottom: 12px;"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path><polyline points="17 8 12 3 7 8"></polyline><line x1="12" y1="3" x2="12" y2="15"></line></svg>
                            <p style="font-size: 0.9rem; color: var(--text-secondary);" id="file-label">Click to select or drag your exported app <strong>.zip</strong> file here</p>
                            <input type="file" id="file-input" name="file" accept=".zip" required style="display: none;" onchange="handleFileSelect(this)">
                        </div>
                    </div>

                    <div class="input-group">
                        <label class="input-label">Application Name</label>
                        <input type="text" name="name" id="name-input" class="text-input" placeholder="e.g. Sleek Pomodoro Timer (Leave empty to use zip name)">
                    </div>

                    <div class="input-group">
                        <label class="input-label">Generation Prompt</label>
                        <textarea name="prompt" required class="text-input" style="height: 90px; resize: vertical;" placeholder="Paste the natural language prompt used to build the app..."></textarea>
                    </div>

                    <div class="input-group" style="margin-bottom: 28px;">
                        <label class="input-label">Description (Optional)</label>
                        <textarea name="description" class="text-input" style="height: 60px; resize: vertical;" placeholder="A brief description of features, libraries used, etc."></textarea>
                    </div>

                    <button type="submit" class="btn-submit" style="width: 100%%; padding: 14px;">Upload & Publish App</button>
                </form>
            </div>
        </div>

        <script>
            function handleFileSelect(input) {
                const label = document.getElementById('file-label');
                const nameInput = document.getElementById('name-input');
                if (input.files && input.files[0]) {
                    const filename = input.files[0].name;
                    label.innerHTML = "Selected: <strong>" + filename + "</strong>";
                    
                    // Pre-fill name input if it is empty
                    if (nameInput && !nameInput.value.trim()) {
                        const base = filename.replace(/\.zip$/i, '').replace(/[-_]/g, ' ');
                        nameInput.value = base.charAt(0).toUpperCase() + base.slice(1);
                    }
                    
                    document.getElementById('drop-zone').style.borderColor = 'var(--accent-color)';
                }
            }
            
            // Drag and drop logic
            const zone = document.getElementById('drop-zone');
            zone.addEventListener('dragover', (e) => {
                e.preventDefault();
                zone.style.borderColor = 'var(--accent-color)';
                zone.style.background = 'rgba(99, 102, 241, 0.04)';
            });
            zone.addEventListener('dragleave', () => {
                zone.style.borderColor = 'rgba(255,255,255,0.08)';
                zone.style.background = 'rgba(0,0,0,0.15)';
            });
            zone.addEventListener('drop', (e) => {
                e.preventDefault();
                zone.style.borderColor = 'rgba(255,255,255,0.08)';
                zone.style.background = 'rgba(0,0,0,0.15)';
                const files = e.dataTransfer.files;
                if (files.length) {
                    const input = document.getElementById('file-input');
                    input.files = files;
                    handleFileSelect(input);
                }
            });
        </script>
    `, errAlert)

	return getHeader("Upload App", user) + body + getFooter()
}

func RenderAppDetail(app *App, user *User, token string) string {
	viewsText := fmt.Sprintf("%d views", app.Views)
	if app.Views == 1 {
		viewsText = "1 view"
	}
	downloadsText := fmt.Sprintf("%d downloads", app.Downloads)
	if app.Downloads == 1 {
		downloadsText = "1 download"
	}

	cliImportCommand := fmt.Sprintf("promptyly import %s.zip", app.ID)
	cliURLCommand := fmt.Sprintf("promptyly handle \"prompt://%s\"", app.ID)

	deleteBtnHTML := ""
	if user != nil && app.UserID == user.ID {
		deleteBtnHTML = fmt.Sprintf(`
                    <button onclick="deleteApp('%s')" class="nav-btn" style="padding: 14px; text-align: center; font-size: 1rem; display: block; width: 100%%; cursor: pointer; background: rgba(239, 68, 68, 0.08); border: 1px solid var(--error-color); color: #fca5a5; margin-top: 12px; transition: all 0.2s;" onmouseover="this.style.background='rgba(239, 68, 68, 0.16)'" onmouseout="this.style.background='rgba(239, 68, 68, 0.08)'">Delete App</button>
        `, app.ID)
	}

	// Build profile token details if owner or auth is present
	apiPublishHelp := ""
	if user != nil {
		apiPublishHelp = fmt.Sprintf(`
        <div class="card" style="margin-top: 24px; padding: 24px;">
            <h3 style="font-size: 1.15rem; margin-bottom: 12px; display: flex; align-items: center; gap: 8px;">
                <svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="color: var(--accent-color);"><rect x="3" y="11" width="18" height="11" rx="2" ry="2"></rect><path d="M7 11V7a5 5 0 0 1 10 0v4"></path></svg>
                Machine Integration Token
            </h3>
            <p style="color: var(--text-secondary); font-size: 0.9rem; line-height: 1.5; margin-bottom: 14px;">
                Your API token is configured to authenticate your CLI client or autonomous agents to publish apps automatically.
            </p>
            <div style="background: rgba(0,0,0,0.3); border: 1px solid var(--border-color); border-radius: 8px; padding: 12px 16px; display: flex; align-items: center; justify-content: space-between; font-family: 'JetBrains Mono', monospace; font-size: 0.85rem;">
                <span style="color: #a5b4fc; word-break: break-all;" id="token-span">%s</span>
                <button class="nav-btn" style="padding: 6px 12px; font-size: 0.8rem; box-shadow: none;" onclick="copyToken()">Copy Token</button>
            </div>
        </div>`, html.EscapeString(user.Token))
	}

	body := fmt.Sprintf(`
        <div style="display: flex; gap: 32px; flex-wrap: wrap;">
            <div style="flex: 2; min-width: 320px; display: flex; flex-direction: column; gap: 28px;">
                <div class="card" style="padding: 32px;">
                    <div style="display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 16px; flex-wrap: wrap; gap: 16px;">
                        <h1 style="font-size: 2.2rem; letter-spacing: -0.02em;">%s</h1>
                        <span style="font-size: 0.85rem; padding: 6px 12px; border-radius: 8px; background: rgba(99, 102, 241, 0.12); color: #a5b4fc; font-weight: 600;">Uploaded by %s</span>
                    </div>

                    <div style="display: flex; gap: 16px; font-size: 0.8rem; color: var(--text-muted); margin-bottom: 28px;">
                        <span>%s</span>
                        <span>&bull;</span>
                        <span>%s</span>
                        <span>&bull;</span>
                        <span>Created %s</span>
                    </div>

                    <h3 style="font-size: 1rem; color: var(--text-secondary); text-transform: uppercase; letter-spacing: 0.05em; margin-bottom: 10px;">Prompt Used to Build This App</h3>
                    <div style="background: rgba(0,0,0,0.2); border-left: 4px solid var(--accent-color); padding: 16px 20px; border-radius: 0 8px 8px 0; font-size: 1rem; line-height: 1.6; color: var(--text-primary); font-style: italic; margin-bottom: 28px;">
                        "%s"
                    </div>

                    %s
                </div>

                <div class="card" style="padding: 28px;">
                    <h3 style="font-size: 1.15rem; margin-bottom: 16px; color: var(--text-primary);">How to Run Locally</h3>
                    <p style="color: var(--text-secondary); font-size: 0.95rem; line-height: 1.6; margin-bottom: 16px;">
                        To import this application into your local Promptyly instance, download the ZIP file and run:
                    </p>
                    
                    <div class="copy-wrapper" style="background: rgba(0,0,0,0.3); border: 1px solid var(--border-color); border-radius: 8px; padding: 12px 16px; display: flex; align-items: center; justify-content: space-between; font-family: 'JetBrains Mono', monospace; font-size: 0.85rem; margin-bottom: 20px;">
                        <span style="color: #a5b4fc;">%s</span>
                        <button class="nav-btn" style="padding: 6px 12px; font-size: 0.8rem; box-shadow: none;" onclick="copyCmd('%s')">Copy</button>
                    </div>

                    <p style="color: var(--text-secondary); font-size: 0.95rem; line-height: 1.6; margin-bottom: 16px;">
                        Alternatively, you can launch it directly from the custom URL protocol handler (if registered):
                    </p>
                    
                    <div class="copy-wrapper" style="background: rgba(0,0,0,0.3); border: 1px solid var(--border-color); border-radius: 8px; padding: 12px 16px; display: flex; align-items: center; justify-content: space-between; font-family: 'JetBrains Mono', monospace; font-size: 0.85rem;">
                        <span style="color: #a5b4fc;">%s</span>
                        <button class="nav-btn" style="padding: 6px 12px; font-size: 0.8rem; box-shadow: none;" onclick="copyCmd('%s')">Copy</button>
                    </div>
                </div>
            </div>

            <div style="flex: 1; min-width: 300px; display: flex; flex-direction: column; gap: 28px;">
                <div class="card" style="padding: 28px; display: flex; flex-direction: column; gap: 18px; text-align: center;">
                    <h3 style="font-size: 1.25rem;">Actions</h3>
                    
                    <a href="/apps/%s/" target="_blank" class="nav-btn" style="padding: 14px; text-align: center; font-size: 1rem; display: block;">Launch Live Website</a>
                    
                    <a href="/api/apps/download/%s" class="nav-link" style="padding: 14px; border: 1px solid var(--border-color); border-radius: 8px; font-weight: 600; font-size: 1rem; display: flex; align-items: center; justify-content: center; gap: 8px; transition: all 0.2s; background: rgba(255,255,255,0.02);" onmouseover="this.style.borderColor='var(--text-secondary)'; this.style.background='rgba(255,255,255,0.04)'" onmouseout="this.style.borderColor='var(--border-color)'; this.style.background='rgba(255,255,255,0.02)'">
                        <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path><polyline points="7 10 12 15 17 10"></polyline><line x1="12" y1="15" x2="12" y2="3"></line></svg>
                        Download Source ZIP
                    </a>

                    %s
                </div>

                <div class="card" style="padding: 28px;">
                    <h3 style="font-size: 1.15rem; margin-bottom: 12px;">App Stats</h3>
                    <div style="display: flex; flex-direction: column; gap: 12px; font-size: 0.95rem; color: var(--text-secondary);">
                        <div style="display: flex; justify-content: space-between; border-bottom: 1px solid rgba(255,255,255,0.04); padding-bottom: 8px;">
                            <span>Views</span>
                            <strong style="color: var(--text-primary);">%d</strong>
                        </div>
                        <div style="display: flex; justify-content: space-between; border-bottom: 1px solid rgba(255,255,255,0.04); padding-bottom: 8px;">
                            <span>Downloads</span>
                            <strong style="color: var(--text-primary);">%d</strong>
                        </div>
                        <div style="display: flex; justify-content: space-between;">
                            <span>Share ID</span>
                            <strong style="color: var(--accent-color); font-family: monospace;">%s</strong>
                        </div>
                    </div>
                </div>

                %s
            </div>
        </div>

        <script>
            function copyCmd(text) {
                navigator.clipboard.writeText(text).then(() => {
                    alert('Command copied to clipboard!');
                });
            }

            function copyToken() {
                const token = document.getElementById('token-span').textContent;
                navigator.clipboard.writeText(token).then(() => {
                    alert('API Token copied to clipboard!');
                });
            }

            function deleteApp(appId) {
                if (confirm('Are you sure you want to permanently delete this application from the remote sharing registry? This cannot be undone.')) {
                    fetch('/api/apps/delete/' + appId, {
                        method: 'POST',
                        headers: {
                            'Content-Type': 'application/json'
                        }
                    }).then(res => {
                        if (res.ok) {
                            alert('Application deleted successfully from registry.');
                            window.location.href = '/';
                        } else {
                            res.text().then(txt => alert('Failed to delete: ' + txt));
                        }
                    }).catch(err => {
                        alert('Error: ' + err);
                    });
                }
            }
        </script>
    `, html.EscapeString(app.Name), html.EscapeString(app.Username), viewsText, downloadsText, app.CreatedAt.Format("Jan 02, 2006"), html.EscapeString(app.Prompt),
		func() string {
			if app.Description != "" {
				return fmt.Sprintf(`
                <h3 style="font-size: 1rem; color: var(--text-secondary); text-transform: uppercase; letter-spacing: 0.05em; margin-bottom: 8px;">Description</h3>
                <p style="color: var(--text-secondary); font-size: 1rem; line-height: 1.6; white-space: pre-line;">%s</p>
                `, html.EscapeString(app.Description))
			}
			return ""
		}(),
		cliImportCommand, cliImportCommand, cliURLCommand, cliURLCommand, app.ID, app.ID, deleteBtnHTML, app.Views, app.Downloads, app.ID, apiPublishHelp)

	return getHeader(app.Name, user) + body + getFooter()
}

func RenderProfile(user *User) string {
	token := user.Token
	maskedToken := ""
	if len(token) > 8 {
		maskedToken = token[:4] + "••••••••••••••••••••••••" + token[len(token)-4:]
	} else {
		maskedToken = token
	}

	body := fmt.Sprintf(`
    <div class="content-container" style="max-width: 800px; margin: 40px auto; padding: 20px;">
        <div style="background: var(--card-bg); border: 1px solid var(--border-color); border-radius: 16px; padding: 32px; backdrop-filter: blur(20px); box-shadow: 0 10px 30px rgba(0, 0, 0, 0.25);">
            <div style="display: flex; align-items: center; gap: 20px; border-bottom: 1px solid var(--border-color); padding-bottom: 24px; margin-bottom: 28px;">
                <div style="width: 64px; height: 64px; background: var(--accent-grad); border-radius: 50%%; display: flex; align-items: center; justify-content: center; font-size: 1.8rem; font-weight: 700; color: white; box-shadow: 0 0 20px var(--accent-glow);">
                    %s
                </div>
                <div>
                    <h1 style="font-size: 1.8rem; font-weight: 700; background: linear-gradient(135deg, #f8fafc 30%%, #94a3b8 100%%); -webkit-background-clip: text; -webkit-text-fill-color: transparent; letter-spacing: -0.02em;">Developer Profile</h1>
                    <p style="color: var(--text-secondary); margin-top: 4px; font-size: 0.95rem;">Manage your Promptyly developer account settings</p>
                </div>
            </div>

            <div style="display: flex; flex-direction: column; gap: 24px;">
                <!-- Username info -->
                <div>
                    <span style="display: block; font-size: 0.75rem; text-transform: uppercase; color: var(--text-muted); font-weight: 700; letter-spacing: 0.05em; margin-bottom: 6px;">Username</span>
                    <div style="font-size: 1.1rem; font-weight: 600; color: var(--text-primary);">%s</div>
                </div>

                <!-- Account Created -->
                <div>
                    <span style="display: block; font-size: 0.75rem; text-transform: uppercase; color: var(--text-muted); font-weight: 700; letter-spacing: 0.05em; margin-bottom: 6px;">Member Since</span>
                    <div style="font-size: 1.0rem; color: var(--text-secondary);">%s</div>
                </div>

                <!-- API Token -->
                <div style="border-top: 1px solid var(--border-color); padding-top: 24px;">
                    <span style="display: block; font-size: 0.75rem; text-transform: uppercase; color: var(--text-muted); font-weight: 700; letter-spacing: 0.05em; margin-bottom: 6px;">Sharing Registry API Token</span>
                    <p style="color: var(--text-secondary); font-size: 0.9rem; margin-bottom: 12px; line-height: 1.5;">Use this token to authenticate when publishing packages from the CLI or third-party clients.</p>
                    
                    <div style="display: flex; flex-direction: column; gap: 10px; background: rgba(0, 0, 0, 0.25); border: 1px solid var(--border-color); border-radius: 8px; padding: 16px;">
                        <div style="display: flex; align-items: center; justify-content: space-between; gap: 15px;">
                            <div style="font-family: 'JetBrains Mono', monospace; font-size: 0.95rem; color: #a5b4fc; word-break: break-all;" id="token-display">%s</div>
                            <button onclick="toggleToken()" id="btn-toggle" class="nav-btn" style="padding: 6px 12px; font-size: 0.8rem; white-space: nowrap; margin-bottom: 0; background: rgba(255, 255, 255, 0.03); border: 1px solid var(--border-color);">Show Token</button>
                        </div>
                        <div style="display: flex; gap: 10px; border-top: 1px solid rgba(255, 255, 255, 0.03); padding-top: 12px; margin-top: 4px;">
                            <button onclick="copyTokenText()" class="nav-btn" style="padding: 8px 16px; font-size: 0.85rem; margin-bottom: 0; background: var(--accent-grad); border: none;">Copy Token</button>
                        </div>
                    </div>
                </div>

                <!-- CLI Integration Help -->
                <div style="background: rgba(99, 102, 241, 0.04); border: 1px solid rgba(99, 102, 241, 0.15); border-radius: 10px; padding: 20px; margin-top: 12px;">
                    <h4 style="font-size: 0.95rem; font-weight: 600; color: #a5b4fc; margin-bottom: 8px; display: flex; align-items: center; gap: 8px;">
                        <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m18 16 4-4-4-4"></path><path d="m6 8-4 4 4 4"></path><path d="m14.5 4-5 16"></path></svg>
                        CLI Integration
                    </h4>
                    <p style="color: var(--text-secondary); font-size: 0.85rem; line-height: 1.5; margin-bottom: 12px;">To configure your local Promptyly CLI to push to this server, execute the following commands:</p>
                    <pre style="background: rgba(0, 0, 0, 0.4); border: 1px solid var(--border-color); border-radius: 6px; padding: 12px; font-family: 'JetBrains Mono', monospace; font-size: 0.8rem; color: #a5b4fc; overflow-x: auto; line-height: 1.4;">promptyly config set sharing_server_url <span id="cli-url"></span>&#10;promptyly config set sharing_token %s</pre>
                </div>
            </div>
        </div>
    </div>

    <script>
        const REAL_TOKEN = "%s";
        const MASKED_TOKEN = "%s";
        let isTokenVisible = false;

        // Set CLI URL based on current window location
        document.getElementById('cli-url').textContent = window.location.origin;

        function toggleToken() {
            const display = document.getElementById('token-display');
            const btn = document.getElementById('btn-toggle');
            isTokenVisible = !isTokenVisible;
            if (isTokenVisible) {
                display.textContent = REAL_TOKEN;
                btn.textContent = 'Hide Token';
            } else {
                display.textContent = MASKED_TOKEN;
                btn.textContent = 'Show Token';
            }
        }

        function copyTokenText() {
            navigator.clipboard.writeText(REAL_TOKEN).then(() => {
                alert('API Token copied to clipboard!');
            }).catch(err => {
                alert('Failed to copy token: ' + err);
            });
        }
    </script>
    `,
		string(user.Username[0]),
		html.EscapeString(user.Username),
		user.CreatedAt.Format("January 02, 2006"),
		maskedToken,
		token,
		token,
		maskedToken,
	)

	return getHeader("Developer Profile", user) + body + getFooter()
}

func RenderAdminPanel(users []*User, currentUser *User, config ServerConfig, analytics []AnalyticsEvent) string {
	var rows string
	for _, u := range users {
		adminBadge := `<span style="background: rgba(255,255,255,0.05); color: var(--text-secondary); padding: 2px 6px; border-radius: 4px; font-size: 0.8rem;">User</span>`
		if u.IsAdmin {
			adminBadge = `<span style="background: rgba(99, 102, 241, 0.15); color: #a5b4fc; padding: 2px 6px; border-radius: 4px; font-size: 0.8rem; font-weight: 600;">Admin</span>`
		}

		approvalBadge := `<span style="background: rgba(239, 84, 84, 0.15); color: var(--error-color); padding: 2px 6px; border-radius: 4px; font-size: 0.8rem; font-weight: 600;">Pending</span>`
		if u.IsApproved {
			approvalBadge = `<span style="background: rgba(16, 185, 129, 0.15); color: var(--success-color); padding: 2px 6px; border-radius: 4px; font-size: 0.8rem; font-weight: 600;">Approved</span>`
		}

		actions := ""
		if u.Username != currentUser.Username {
			approveBtn := ""
			if !u.IsApproved {
				approveBtn = fmt.Sprintf(`<button class="btn-submit" style="padding: 6px 12px; font-size: 0.8rem; border-radius: 4px;" onclick="approveUser('%s')">Approve</button>`, u.Username)
			}
			rejectBtn := fmt.Sprintf(`<button class="btn-submit" style="background: var(--error-color); padding: 6px 12px; font-size: 0.8rem; border-radius: 4px; margin-left: 8px;" onclick="rejectUser('%s')">Delete</button>`, u.Username)
			actions = approveBtn + rejectBtn
		} else {
			actions = `<span style="color: var(--text-muted); font-size: 0.85rem;">Current Admin</span>`
		}

		rows += fmt.Sprintf(`
            <tr style="border-bottom: 1px solid var(--border-color);">
                <td style="padding: 14px; font-weight: 500;">%s</td>
                <td style="padding: 14px; color: var(--text-secondary);">%s</td>
                <td style="padding: 14px;">%s</td>
                <td style="padding: 14px;">%s</td>
                <td style="padding: 14px; text-align: right;">%s</td>
            </tr>
        `, html.EscapeString(u.Username), u.CreatedAt.Format("2006-01-02 15:04"), adminBadge, approvalBadge, actions)
	}

	var pageViews, linkDownloads, appViews, uploads, searches int
	for _, e := range analytics {
		switch e.Category {
		case "page":
			pageViews++
		case "link":
			linkDownloads++
		case "app":
			appViews++
		case "upload":
			uploads++
		case "url":
			searches++
		}
	}

	var eventRows string
	startIdx := len(analytics) - 1
	count := 0
	for i := startIdx; i >= 0 && count < 50; i-- {
		e := analytics[i]
		eventRows += fmt.Sprintf(`
            <tr style="border-bottom: 1px solid var(--border-color); font-size: 0.85rem;">
                <td style="padding: 10px 14px; font-family: 'JetBrains Mono', monospace; color: var(--text-muted);">%s</td>
                <td style="padding: 10px 14px;"><span style="background: rgba(255,255,255,0.04); padding: 2px 6px; border-radius: 4px; font-weight: 600;">%s</span></td>
                <td style="padding: 10px 14px; font-weight: 500;">%s</td>
                <td style="padding: 10px 14px; color: var(--text-secondary); max-width: 200px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;" title="%s">%s</td>
                <td style="padding: 10px 14px; color: var(--text-muted);">%s</td>
            </tr>
        `, e.Timestamp.Format("15:04:05"), html.EscapeString(e.Category), html.EscapeString(e.Action), html.EscapeString(e.Label), html.EscapeString(e.Label), html.EscapeString(e.IP))
		count++
	}
	if eventRows == "" {
		eventRows = `<tr><td colspan="5" style="padding: 20px; text-align: center; color: var(--text-muted);">No analytics recorded yet.</td></tr>`
	}

	chkRequireApproval := ""
	if config.RequireAdminApproval {
		chkRequireApproval = "checked"
	}
	chkRequireLogin := ""
	if config.RequireLoginToView {
		chkRequireLogin = "checked"
	}
	chkAllowSelfReg := ""
	if config.AllowSelfRegistration {
		chkAllowSelfReg = "checked"
	}

	body := fmt.Sprintf(`
        <div style="max-width: 900px; margin: 40px auto; padding: 0 24px; flex-grow: 1; width: 100%%;">
            <!-- Analytics Dashboard -->
            <div class="card" style="padding: 36px; border-radius: 12px; margin-bottom: 32px;">
                <h2 style="font-size: 1.8rem; margin-bottom: 8px;">Analytics Dashboard</h2>
                <p style="color: var(--text-secondary); font-size: 0.95rem; margin-bottom: 24px;">Real-time traffic and event tracking statistics on this registry.</p>
                
                <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(140px, 1fr)); gap: 16px; margin-bottom: 32px;">
                    <div style="background: rgba(255, 255, 255, 0.02); border: 1px solid var(--border-color); border-radius: 8px; padding: 16px; text-align: center;">
                        <span style="font-size: 2rem; display: block; margin-bottom: 6px;">📄</span>
                        <strong style="font-size: 1.5rem; color: #a5b4fc; display: block;">%d</strong>
                        <span style="font-size: 0.8rem; color: var(--text-secondary);">Page Views</span>
                    </div>
                    <div style="background: rgba(255, 255, 255, 0.02); border: 1px solid var(--border-color); border-radius: 8px; padding: 16px; text-align: center;">
                        <span style="font-size: 2rem; display: block; margin-bottom: 6px;">🔗</span>
                        <strong style="font-size: 1.5rem; color: #f43f5e; display: block;">%d</strong>
                        <span style="font-size: 0.8rem; color: var(--text-secondary);">Link Downloads</span>
                    </div>
                    <div style="background: rgba(255, 255, 255, 0.02); border: 1px solid var(--border-color); border-radius: 8px; padding: 16px; text-align: center;">
                        <span style="font-size: 2rem; display: block; margin-bottom: 6px;">🤖</span>
                        <strong style="font-size: 1.5rem; color: #10b981; display: block;">%d</strong>
                        <span style="font-size: 0.8rem; color: var(--text-secondary);">App Events</span>
                    </div>
                    <div style="background: rgba(255, 255, 255, 0.02); border: 1px solid var(--border-color); border-radius: 8px; padding: 16px; text-align: center;">
                        <span style="font-size: 2rem; display: block; margin-bottom: 6px;">📤</span>
                        <strong style="font-size: 1.5rem; color: #fbbf24; display: block;">%d</strong>
                        <span style="font-size: 0.8rem; color: var(--text-secondary);">Uploaded Apps</span>
                    </div>
                    <div style="background: rgba(255, 255, 255, 0.02); border: 1px solid var(--border-color); border-radius: 8px; padding: 16px; text-align: center;">
                        <span style="font-size: 2rem; display: block; margin-bottom: 6px;">🔍</span>
                        <strong style="font-size: 1.5rem; color: #38bdf8; display: block;">%d</strong>
                        <span style="font-size: 0.8rem; color: var(--text-secondary);">Registry Queries</span>
                    </div>
                </div>

                <h3 style="font-size: 1.2rem; margin-bottom: 16px;">Recent Event Logs (Last 50)</h3>
                <div style="overflow-x: auto; background: rgba(0,0,0,0.2); border-radius: 8px; border: 1px solid var(--border-color);">
                    <table style="width: 100%%; border-collapse: collapse; text-align: left; font-size: 0.95rem;">
                        <thead>
                            <tr style="border-bottom: 1px solid var(--border-color); background: rgba(255,255,255,0.02); color: var(--text-secondary);">
                                <th style="padding: 10px 14px;">Time</th>
                                <th style="padding: 10px 14px;">Category</th>
                                <th style="padding: 10px 14px;">Action</th>
                                <th style="padding: 10px 14px;">Label/Target</th>
                                <th style="padding: 10px 14px;">IP Address</th>
                            </tr>
                        </thead>
                        <tbody>
                            %s
                        </tbody>
                    </table>
                </div>
            </div>

            <!-- Registry Settings -->
            <div class="card" style="padding: 36px; border-radius: 12px; margin-bottom: 32px;">
                <h2 style="font-size: 1.8rem; margin-bottom: 8px;">Registry Settings</h2>
                <p style="color: var(--text-secondary); font-size: 0.95rem; margin-bottom: 24px;">Configure access and security policies for this registry server.</p>
                
                <div style="display: flex; flex-direction: column; gap: 18px;">
                    <div style="display: flex; align-items: flex-start; gap: 12px;">
                        <input type="checkbox" id="cfg-require-approval" %s style="margin-top: 4px; width: 18px; height: 18px; accent-color: var(--accent-color); cursor: pointer;">
                        <div>
                            <label for="cfg-require-approval" style="font-weight: 600; cursor: pointer; color: var(--text-primary);">Require Admin Approval</label>
                            <p style="color: var(--text-muted); font-size: 0.85rem; margin-top: 2px;">Newly registered accounts must be approved by an administrator before they can log in.</p>
                        </div>
                    </div>

                    <div style="display: flex; align-items: flex-start; gap: 12px;">
                        <input type="checkbox" id="cfg-require-login" %s style="margin-top: 4px; width: 18px; height: 18px; accent-color: var(--accent-color); cursor: pointer;">
                        <div>
                            <label for="cfg-require-login" style="font-weight: 600; cursor: pointer; color: var(--text-primary);">Require Login to View</label>
                            <p style="color: var(--text-muted); font-size: 0.85rem; margin-top: 2px;">Enforce authentication. Guests cannot view the app registry or homepage installation instructions.</p>
                        </div>
                    </div>

                    <div style="display: flex; align-items: flex-start; gap: 12px;">
                        <input type="checkbox" id="cfg-allow-selfreg" %s style="margin-top: 4px; width: 18px; height: 18px; accent-color: var(--accent-color); cursor: pointer;">
                        <div>
                            <label for="cfg-allow-selfreg" style="font-weight: 600; cursor: pointer; color: var(--text-primary);">Allow Self-Registration</label>
                            <p style="color: var(--text-muted); font-size: 0.85rem; margin-top: 2px;">Allow new users to sign up and create accounts using the main page form.</p>
                        </div>
                    </div>
                </div>

                <div style="margin-top: 28px; display: flex; align-items: center; gap: 16px;">
                    <button class="btn-submit" onclick="saveSettings()" style="padding: 10px 24px; font-size: 0.9rem;">Save Settings</button>
                    <span id="settings-status" style="font-size: 0.85rem; font-weight: 500; transition: opacity 0.3s; opacity: 0;"></span>
                </div>
            </div>

            <!-- User Management -->
            <div class="card" style="padding: 36px; border-radius: 12px;">
                <h2 style="font-size: 1.8rem; margin-bottom: 8px;">User Management</h2>
                <p style="color: var(--text-secondary); font-size: 0.95rem; margin-bottom: 32px;">Manage registered developer accounts and pending approvals.</p>

                <div style="overflow-x: auto; background: rgba(0,0,0,0.2); border-radius: 8px; border: 1px solid var(--border-color);">
                    <table style="width: 100%%; border-collapse: collapse; text-align: left; font-size: 0.95rem;">
                        <thead>
                            <tr style="border-bottom: 1px solid var(--border-color); background: rgba(255,255,255,0.02); color: var(--text-secondary);">
                                <th style="padding: 14px;">Username</th>
                                <th style="padding: 14px;">Created At</th>
                                <th style="padding: 14px;">Role</th>
                                <th style="padding: 14px;">Status</th>
                                <th style="padding: 14px; text-align: right;">Actions</th>
                            </tr>
                        </thead>
                        <tbody>
                            %s
                        </tbody>
                    </table>
                </div>
            </div>
        </div>

        <script>
            async function saveSettings() {
                const requireApproval = document.getElementById('cfg-require-approval').checked;
                const requireLogin = document.getElementById('cfg-require-login').checked;
                const allowSelfReg = document.getElementById('cfg-allow-selfreg').checked;
                const statusEl = document.getElementById('settings-status');

                statusEl.textContent = "Saving...";
                statusEl.style.color = "var(--text-secondary)";
                statusEl.style.opacity = 1;

                try {
                    const resp = await fetch("/api/admin/config", {
                        method: "POST",
                        headers: { "Content-Type": "application/json" },
                        body: JSON.stringify({
                            require_admin_approval: requireApproval,
                            require_login_to_view: requireLogin,
                            allow_self_registration: allowSelfReg
                        })
                    });
                    const res = await resp.json();
                    if (res.success) {
                        statusEl.textContent = "✓ Settings saved successfully!";
                        statusEl.style.color = "var(--success-color)";
                        setTimeout(() => {
                            statusEl.style.opacity = 0;
                        }, 3000);
                    } else {
                        statusEl.textContent = "Error: " + res.error;
                        statusEl.style.color = "var(--error-color)";
                    }
                } catch (e) {
                    statusEl.textContent = "Save failed: " + e;
                    statusEl.style.color = "var(--error-color)";
                }
            }

            async function approveUser(username) {
                if (!confirm("Are you sure you want to approve user " + username + "?")) return;
                try {
                    const resp = await fetch("/api/admin/approve", {
                        method: "POST",
                        headers: { "Content-Type": "application/json" },
                        body: JSON.stringify({ username })
                    });
                    const res = await resp.json();
                    if (res.success) {
                        location.reload();
                    } else {
                        alert("Error: " + res.error);
                    }
                } catch (e) {
                    alert("Approve failed: " + e);
                }
            }

            async function rejectUser(username) {
                if (!confirm("Are you sure you want to delete user " + username + "? This action is irreversible.")) return;
                try {
                    const resp = await fetch("/api/admin/reject", {
                        method: "POST",
                        headers: { "Content-Type": "application/json" },
                        body: JSON.stringify({ username })
                    });
                    const res = await resp.json();
                    if (res.success) {
                        location.reload();
                    } else {
                        alert("Error: " + res.error);
                    }
                } catch (e) {
                    alert("Reject failed: " + e);
                }
            }
        </script>
    `, pageViews, linkDownloads, appViews, uploads, searches, eventRows, chkRequireApproval, chkRequireLogin, chkAllowSelfReg, rows)

	return getHeader("Admin Panel", currentUser) + body + getFooter()
}
