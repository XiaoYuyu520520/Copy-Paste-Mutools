# import requests
# import json
# import sys

# # 配置
# AI_API_KEY = os.environ.get("AI_API_KEY", "")  # 从环境变量读取，不要把密钥写进代码
# AI_BASE_URL = "https://api.huandutech.com/v1"
# # 选一个测试成功的模型，想看逻辑推导过程就选带 thinking 的
# AI_MODEL = "gemini-2.5-pro-c-thinking" 

# def chat():
#     url = f"{AI_BASE_URL}/chat/completions"
#     headers = {
#         "Authorization": f"Bearer {AI_API_KEY}",
#         "Content-Type": "application/json"
#     }
    
#     messages = [{"role": "system", "content": "你是一个专业且幽默的 AI 助手。"}]
    
#     print(f"--- 已连接到 {AI_MODEL} (输入 'exit' 退出) ---")

#     while True:
#         user_input = input("\n👤 你: ")
#         if user_input.lower() in ['exit', 'quit', '退出']:
#             break
            
#         messages.append({"role": "user", "content": user_input})
        
#         data = {
#             "model": AI_MODEL,
#             "messages": messages,
#             "stream": True  # 开启流式传输，体验更丝滑
#         }

#         print("🤖 AI: ", end="", flush=True)
        
#         try:
#             response = requests.post(url, headers=headers, json=data, stream=True)
#             full_response = ""
            
#             for line in response.iter_lines():
#                 if line:
#                     # 移除 "data: " 前缀
#                     line_text = line.decode('utf-8')
#                     if line_text.startswith("data: "):
#                         line_text = line_text[6:]
                    
#                     if line_text == "[DONE]":
#                         break
                        
#                     try:
#                         chunk = json.loads(line_text)
#                         content = chunk['choices'][0]['delta'].get('content', '')
#                         print(content, end="", flush=True)
#                         full_response += content
#                     except:
#                         continue
            
#             print() # 换行
#             messages.append({"role": "assistant", "content": full_response})
            
#         except Exception as e:
#             print(f"\n❌ 出错了: {e}")

# if __name__ == "__main__":
#     chat()














# import requests
# import json

# # 配置
# AI_API_KEY = os.environ.get("AI_API_KEY", "")  # 从环境变量读取，不要把密钥写进代码
# AI_BASE_URL = "https://api.huandutech.com/v1"

# # 你获取到的模型列表
# MODELS_TO_TEST = [
#     "claude-haiku-4-5-c",
#     "claude-opus-4-6-c",
#     "claude-sonnet-4-6-c",
#     "gemini-2.5-pro-c",
#     "gemini-2.5-pro-c-thinking",
#     "gemini-3.1-pro-c",
#     "gemini-3-pro-preview-c",
#     "gemini-3-pro-preview-c-thinking"
# ]

# def check_model(model_name):
#     url = f"{AI_BASE_URL}/chat/completions"
#     headers = {
#         "Authorization": f"Bearer {AI_API_KEY}",
#         "Content-Type": "application/json"
#     }
#     data = {
#         "model": model_name,
#         "messages": [{"role": "user", "content": "hi"}],
#         "max_tokens": 5
#     }

#     try:
#         response = requests.post(url, headers=headers, json=data, timeout=10)
#         if response.status_code == 200:
#             return True, "✅ 可用"
#         else:
#             # 提取错误详情
#             try:
#                 err_msg = response.json().get('error', {}).get('message', '未知错误')
#             except:
#                 err_msg = response.text[:50]
#             return False, f"❌ 失败 (状态码 {response.status_code}: {err_msg})"
#     except Exception as e:
#         return False, f"⚠️ 连接异常: {str(e)}"

# def run_batch_test():
#     print(f"🚀 开始测试 API 可用性...\n{'='*50}")
#     results = []
    
#     for model in MODELS_TO_TEST:
#         print(f"正在测试: {model:35}", end="", flush=True)
#         is_ok, status = check_model(model)
#         print(status)
#         if is_ok:
#             results.append(model)
    
#     print(f"\n{'='*50}\n📊 测试完成！")
#     if results:
#         print(f"以下模型当前可正常运行: \n  - " + "\n  - ".join(results))
#     else:
#         print("❌ 遗憾，所有模型均返回错误。请检查 API Key 余额或联系服务商。")

# if __name__ == "__main__":
#     run_batch_test()






















# import requests
# import json

# # 请务必更新模型名称为列表中的一个
# AI_API_KEY = os.environ.get("AI_API_KEY", "")  # 从环境变量读取，不要把密钥写进代码
# AI_BASE_URL = "https://api.huandutech.com/v1"
# AI_MODEL = "gemini-3.1-pro-c"  # 改为列表中存在的模型名

# def test_gemini_api():
#     url = f"{AI_BASE_URL}/chat/completions"
#     headers = {
#         "Authorization": f"Bearer {AI_API_KEY}",
#         "Content-Type": "application/json"
#     }
#     data = {
#         "model": AI_MODEL,
#         "messages": [{"role": "user", "content": "你好，请确认你的模型版本。"}],
#         "temperature": 0.7
#     }

#     print(f"正在请求最新模型: {AI_MODEL} ...")
#     try:
#         response = requests.post(url, headers=headers, json=data)
#         if response.status_code == 200:
#             print("\n✅ 测试成功！")
#             print(f"模型回复: {response.json()['choices'][0]['message']['content']}")
#         else:
#             print(f"\n❌ 请求失败，状态码: {response.status_code}")
#             print(f"错误信息: {response.text}")
#     except Exception as e:
#         print(f"发生异常: {e}")

# if __name__ == "__main__":
#     test_gemini_api()








import os
import requests

AI_API_KEY = os.environ.get("AI_API_KEY", "")  # 从环境变量读取，不要把密钥写进代码
AI_BASE_URL = "https://api.huandutech.com/v1"

def list_models():
    url = f"{AI_BASE_URL}/models"
    headers = {"Authorization": f"Bearer {AI_API_KEY}"}
    
    response = requests.get(url, headers=headers)
    if response.status_code == 200:
        models = response.json()
        print("当前可用模型列表：")
        for m in models.get('data', []):
            print(f"- {m['id']}")
    else:
        print(f"查询失败: {response.text}")

list_models()





















# import requests
# import json

# # 配置信息
# AI_API_KEY = os.environ.get("AI_API_KEY", "")  # 从环境变量读取，不要把密钥写进代码
# AI_BASE_URL = "https://api.huandutech.com/v1"
# AI_MODEL = "gemini-2.5-flash-nothinking"

# def test_gemini_api():
#     url = f"{AI_BASE_URL}/chat/completions"
    
#     headers = {
#         "Authorization": f"Bearer {AI_API_KEY}",
#         "Content-Type": "application/json"
#     }
    
#     data = {
#         "model": AI_MODEL,
#         "messages": [
#             {"role": "user", "content": "你好！如果你收到了这条消息，请回复：测试成功。"}
#         ],
#         "temperature": 0.7
#     }

#     print(f"正在请求模型: {AI_MODEL} ...")
    
#     try:
#         response = requests.post(url, headers=headers, data=json.dumps(data))
        
#         # 检查 HTTP 状态码
#         if response.status_code == 200:
#             result = response.json()
#             answer = result['choices'][0]['message']['content']
#             print("\n--- 响应成功 ---")
#             print(f"模型回复: {answer}")
#         else:
#             print("\n--- 请求失败 ---")
#             print(f"状态码: {response.status_code}")
#             print(f"错误详情: {response.text}")
            
#     except Exception as e:
#         print(f"\n发生错误: {e}")

# if __name__ == "__main__":
#     test_gemini_api()