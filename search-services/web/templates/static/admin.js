async function login() {
    const name = document.getElementById('username').value;
    const password = document.getElementById('password').value;

    const response = await fetch(`${API_BASE}/api/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name, password })
    });

    if (response.ok) {
        const token = await response.text();
        localStorage.setItem('xkcd_token', token);
        showPanel();
    } else {
        alert("Ошибка авторизации!");
    }
}

function showPanel() {
    document.getElementById('authBlock').style.display = 'none';
    document.getElementById('adminPanel').style.display = 'block';
    refreshStatus();
    setInterval(refreshStatus, 1000);
}

async function runDrop() {
    if (!confirm("Вы уверены, что хотите полностью очистить базу данных?")) {
        return;
    }

    const token = localStorage.getItem('xkcd_token');
    
    try {
        const response = await fetch(`${API_BASE}/api/db`, {
            method: 'DELETE',
            headers: { 
                'Authorization': `Token ${token}` 
            }
        });
        
        if (response.status === 401) {
            alert("Сессия истекла");
            logout();
        } else if (response.ok) {
            alert("База данных успешно очищена");
            refreshStatus();
        } else {
            alert("Ошибка при удалении базы");
        }
    } catch (e) {
        console.error("Ошибка при отправке запроса Drop:", e);
    }
}

async function refreshStatus() {
    try {
        const statusRes = await fetch(`${API_BASE}/api/db/status`);
        if (statusRes.ok) {
            const statusData = await statusRes.json();
            document.getElementById('statusValue').innerText = statusData.status;
        }

        const statsRes = await fetch(`${API_BASE}/api/db/stats`);
        if (statsRes.ok) {
            const statsData = await statsRes.json();
            document.getElementById('dbCount').innerText = statsData.comics_fetched;
        }
    } catch (e) {
        console.error("Ошибка обновления данных:", e);
    }
}

async function runUpdate() {
    const token = localStorage.getItem('xkcd_token');
    const response = await fetch(`${API_BASE}/api/db/update`, {
        method: 'POST',
        headers: {
            'Authorization': `Token ${token}` 
        }
    });
    
    if (response.status === 401) {
        alert("Сессия истекла или нет доступа");
        logout();
    } else {
        alert("Обновление запущено");
    }
}

async function refreshStats() {
    try {
        const response = await fetch('/api/db/stats');
        const data = await response.json();
        
        const countElement = document.getElementById('dbCount');
        if (countElement) {
            countElement.innerText = data.comics_fetched; 
        }
    } catch (err) {
        console.error("Ошибка загрузки статистики:", err);
    }
}

function logout() {
    localStorage.removeItem('xkcd_token');
    location.reload();
}

let API_BASE = "";

async function initConfig() {
    try {
        const response = await fetch('/api/config'); 
        const config = await response.json();
        
        API_BASE = config.api_url; 
        
        if (API_BASE.includes("api:8080")) {
            API_BASE = API_BASE.replace("api:8080", "localhost:28080");
        }
        console.log("Resolved API Base:", API_BASE);
    } catch (e) {
        console.error("Failed to load API config", e);
    }
}

initConfig();