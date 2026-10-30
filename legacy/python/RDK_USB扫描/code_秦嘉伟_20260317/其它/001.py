import requests
import time
import json
import sys

# 解决 Windows 终端可能出现的中文字符乱码问题
sys.stdout.reconfigure(encoding='utf-8')

# 配置信息
API_KEY = "gg-gcli-9eAHnFdEhQNv-p7YAGB58rBFjzn_WMSeM33QKGpLQPg" # 如果失效请替换为你最新的 Key
BASE_URL = "https://gcli.ggchan.dev/v1/chat/completions"

# 完全按照截图提取的模型列表 (排除了假流式前缀，直接测试底层核心模型)
models_to_test = [
    "gemini-2.5-flash-search",
    "gemini-2.5-pro-maxthinking",
    "gemini-2.5-pro-nothinking",
    "gemini-2.5-pro-search",
    "gemini-3-flash-preview",
    "gemini-3-flash-preview-search",
    "gemini-3-pro-preview",
    "gemini-3-pro-preview-low",
    "gemini-3-pro-preview-search",
    "gemini-3.1-pro-preview",
    "gemini-3.1-pro-preview-low",
    "gemini-3.1-pro-preview-search"
]

def test_model_availability(model_name):
    headers = {
        "Content-Type": "application/json",
        "Authorization": f"Bearer {API_KEY}"
    }
    
    # 测试题：考察逻辑能力
    payload = {
        "model": model_name,
        "messages": [{"role": "user", "content": "1.5和1.11哪个大？简要说明理由。"}],
        "max_tokens": 150
    }

    start_time = time.time()
    try:
        response = requests.post(BASE_URL, headers=headers, json=payload, timeout=20) # 增加了超时时间，因为 maxthinking 模型可能需要更久
        latency = round(time.time() - start_time, 2)
        
        if response.status_code == 200:
            result = response.json()
            
            # 兼容解析
            if "choices" in result:
                answer = result["choices"][0]["message"]["content"].strip()
            elif "candidates" in result:
                answer = result["candidates"][0]["content"]["parts"][0]["text"].strip()
            else:
                answer = "未知返回结构"

            # 把换行符替换掉，让打印更整洁
            answer_preview = answer.replace('\n', '  ')[:50] + "..." if len(answer) > 50 else answer.replace('\n', ' ')
            
            print(f"✅ [{model_name}] 可用 | 耗时: {latency}s")
            print(f"   └ 答复: {answer_preview}")
            return True
        else:
            print(f"❌ [{model_name}] 失败 | 状态码: {response.status_code}")
            return False
            
    except Exception as e:
        print(f"⚠️ [{model_name}] 异常: {str(e)}")
        return False

if __name__ == "__main__":
    print("--- 开始测试图中的 Gemini 模型全家桶 ---\n")
    available_count = 0
    
    for model in models_to_test:
        if test_model_availability(model):
            available_count += 1
            
    print(f"\n--- 测试完成 | 可用模型数量: {available_count}/{len(models_to_test)} ---")