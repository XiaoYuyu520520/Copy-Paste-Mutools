// js/app_3.js
document.addEventListener('DOMContentLoaded', () => {
   // 新的智能代码行
    const API_BASE_URL = `${window.location.protocol}//${window.location.hostname}:${window.location.port}/api`;
    const fileListElement = document.getElementById('file-list');
    const breadcrumbElement = document.getElementById('breadcrumb');
    const actionBar = document.getElementById('action-bar');
    const downloadBtn = document.getElementById('batch-download-btn');

    let selectedItems = new Set();

    function updateActionbar() {
        if (selectedItems.size > 0) {
            actionBar.classList.remove('hidden');
            downloadBtn.textContent = `📥 下载选中的 ${selectedItems.size} 项`;
        } else {
            actionBar.classList.add('hidden');
        }
    }

    async function fetchFiles(path = '') {
        selectedItems.clear();
        updateActionbar();
        fileListElement.innerHTML = '<div class="loading">正在加载...</div>';
        try {
            const response = await fetch(`${API_BASE_URL}/files/${path}`);
            if (!response.ok) throw new Error(`HTTP 错误! 状态: ${response.status}`);
            const files = await response.json();
            renderFiles(files);
            updateBreadcrumb(path);
        } catch (error) {
            fileListElement.innerHTML = `<div class="loading">加载失败: ${error.message}</div>`;
        }
    }

    function renderFiles(files) {
        fileListElement.innerHTML = '';
        if (files.length === 0) {
            fileListElement.innerHTML = '<div class="loading">文件夹为空</div>'; return;
        }

        files.forEach(file => {
            const itemElement = document.createElement('div');
            itemElement.className = 'file-item';
            itemElement.dataset.path = file.path;

            const icon = file.is_directory ? '📁' : '📄';

            itemElement.innerHTML = `
                <input type="checkbox" class="checkbox">
                <span class="icon">${icon}</span>
                <div class="name-link">${file.name}</div>
            `;

            const checkbox = itemElement.querySelector('.checkbox');
            checkbox.addEventListener('change', () => {
                if (checkbox.checked) {
                    selectedItems.add(file.path);
                    itemElement.classList.add('selected');
                } else {
                    selectedItems.delete(file.path);
                    itemElement.classList.remove('selected');
                }
                updateActionbar();
            });

            if (file.is_directory) {
                itemElement.querySelector('.name-link').addEventListener('click', () => fetchFiles(file.path));
            }

            fileListElement.appendChild(itemElement);
        });
    }

    downloadBtn.addEventListener('click', async () => {
        if (selectedItems.size === 0) return;

        const originalText = downloadBtn.textContent;
        downloadBtn.textContent = '正在打包...';
        downloadBtn.disabled = true;

        try {
            const response = await fetch(`${API_BASE_URL}/download/batch`, {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({ paths: Array.from(selectedItems) })
            });

            if (response.ok) {
                const blob = await response.blob();
                const url = window.URL.createObjectURL(blob);
                const a = document.createElement('a');
                a.style.display = 'none';
                a.href = url;
                a.download = 'shared_files.zip';
                document.body.appendChild(a);
                a.click();
                window.URL.revokeObjectURL(url);
                a.remove();
            } else {
                alert('下载失败!');
            }
        } catch (error) {
            console.error('下载出错:', error);
            alert('下载出错!');
        } finally {
            downloadBtn.textContent = originalText;
            downloadBtn.disabled = false;
        }
    });

    // Breadcrumb logic remains the same
    function updateBreadcrumb(path) {
        breadcrumbElement.innerHTML = '<a href="#" data-path="">根目录</a>';
        const parts = path.split('/').filter(p => p);
        let currentPath = '';
        parts.forEach(part => {
            currentPath += `${part}/`;
            breadcrumbElement.innerHTML += `<span>/</span><a href="#" data-path="${currentPath.slice(0, -1)}">${part}</a>`;
        });
        breadcrumbElement.querySelectorAll('a').forEach(a => {
            a.addEventListener('click', (e) => { e.preventDefault(); fetchFiles(e.target.dataset.path); });
        });
    }

    fetchFiles();
});