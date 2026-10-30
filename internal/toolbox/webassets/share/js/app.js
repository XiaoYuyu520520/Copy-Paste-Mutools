document.addEventListener('DOMContentLoaded', () => {
    const API_BASE_URL = `${window.location.protocol}//${window.location.hostname}:8000/api`;
    const fileListElement = document.getElementById('file-list');
    const breadcrumbElement = document.getElementById('breadcrumb');

    async function fetchFiles(path = '') {
        fileListElement.innerHTML = '<div class="loading">正在加载...</div>';
        try {
            const response = await fetch(`${API_BASE_URL}/files/${path}`);
            if (!response.ok) {
                throw new Error(`HTTP 错误! 状态: ${response.status}`);
            }
            const files = await response.json();
            renderFiles(files, path);
            updateBreadcrumb(path);
        } catch (error) {
            fileListElement.innerHTML = `<div class="loading">加载失败: ${error.message}</div>`;
            console.error('获取文件列表失败:', error);
        }
    }

    function renderFiles(files, currentPath) {
        fileListElement.innerHTML = '';
        if (files.length === 0) {
            fileListElement.innerHTML = '<div class="loading">文件夹为空</div>';
            return;
        }

        files.forEach(file => {
            const itemElement = document.createElement('div');
            itemElement.className = 'file-item';
            
            const icon = file.is_directory ? '📁' : '📄';
            
            if (file.is_directory) {
                itemElement.innerHTML = `
                    <span class="icon">${icon}</span>
                    <span class="name">${file.name}</span>
                `;
                itemElement.addEventListener('click', () => fetchFiles(file.path));
            } else {
                itemElement.innerHTML = `
                    <span class="icon">${icon}</span>
                    <a class="name" href="${API_BASE_URL}/download/${file.path}" target="_blank">${file.name}</a>
                `;
            }
            fileListElement.appendChild(itemElement);
        });
    }

    function updateBreadcrumb(path) {
        breadcrumbElement.innerHTML = '<a href="#" data-path="">根目录</a>';
        const parts = path.split('/').filter(p => p);
        let currentPath = '';
        parts.forEach(part => {
            currentPath += `${part}/`;
            breadcrumbElement.innerHTML += `<span>/</span><a href="#" data-path="${currentPath.slice(0, -1)}">${part}</a>`;
        });
        
        breadcrumbElement.querySelectorAll('a').forEach(a => {
            a.addEventListener('click', (e) => {
                e.preventDefault();
                fetchFiles(e.target.dataset.path);
            });
        });
    }
    
    // 初始加载
    fetchFiles();
});