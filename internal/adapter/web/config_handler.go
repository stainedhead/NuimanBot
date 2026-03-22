package web

import (
	"net/http"
)

// handleLLMConfig displays LLM configuration page
func (s *Server) handleLLMConfig(w http.ResponseWriter, r *http.Request) {
	// Check authentication
	user := s.getCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/admin/login", http.StatusFound)
		return
	}

	// Simplified LLM config page
	html := `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>LLM Configuration - NuimanBot Admin</title>
    <script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="bg-gray-100 min-h-screen">
    <div class="container mx-auto px-4 py-8">
        <div class="bg-white rounded-lg shadow-md p-6">
            <h1 class="text-3xl font-bold text-gray-900 mb-6">LLM Configuration</h1>
            <p class="text-gray-600">Configure LLM providers and models here.</p>
            <div class="mt-6">
                <a href="/admin/dashboard" class="btn-secondary">Back to Dashboard</a>
            </div>
        </div>
    </div>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(html))
}

// handleServerConfig displays server configuration page
func (s *Server) handleServerConfig(w http.ResponseWriter, r *http.Request) {
	// Check authentication
	user := s.getCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/admin/login", http.StatusFound)
		return
	}

	// Simplified server config page
	html := `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>Server Configuration - NuimanBot Admin</title>
    <script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="bg-gray-100 min-h-screen">
    <div class="container mx-auto px-4 py-8">
        <div class="bg-white rounded-lg shadow-md p-6">
            <h1 class="text-3xl font-bold text-gray-900 mb-6">Server Configuration</h1>
            <p class="text-gray-600">Configure server settings, paths, and gateways here.</p>
            <div class="mt-6">
                <a href="/admin/dashboard" class="btn-secondary">Back to Dashboard</a>
            </div>
        </div>
    </div>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(html))
}

// handleLogs displays activity log viewer
func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	// Check authentication
	user := s.getCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/admin/login", http.StatusFound)
		return
	}

	// Simplified logs page
	html := `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>Activity Logs - NuimanBot Admin</title>
    <script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="bg-gray-100 min-h-screen">
    <div class="container mx-auto px-4 py-8">
        <div class="bg-white rounded-lg shadow-md p-6">
            <h1 class="text-3xl font-bold text-gray-900 mb-6">Activity Logs</h1>
            <p class="text-gray-600">View and filter system activity logs here.</p>
            <div class="mt-6">
                <a href="/admin/dashboard" class="btn-secondary">Back to Dashboard</a>
            </div>
        </div>
    </div>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(html))
}

// handleAdminIndex redirects to dashboard
func (s *Server) handleAdminIndex(w http.ResponseWriter, r *http.Request) {
	// Check authentication
	user := s.getCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/admin/login", http.StatusFound)
		return
	}

	// Redirect to dashboard
	http.Redirect(w, r, "/admin/dashboard", http.StatusFound)
}
