# ROS2 Helper - 话题扫描工具

ROS2话题扫描工具，支持QOS解析、消息内容解析、光标选择、实时监控和结果保存。

## 功能特性

- **话题扫描**: 扫描活跃话题，获取发布者/订阅者信息
- **QOS解析**: 解析reliability, durability, history, deadline等QOS配置
- **消息解析**: 实际订阅话题获取首条消息，识别数据格式（image, imu, radar, odometry等）
- **光标选择**: 支持prompt_toolkit交互式选择（支持tty环境）
- **实时监控**: 持续扫描话题，支持自动刷新
- **结果保存**: 支持JSON和YAML格式导出

## 快速开始

```bash
# 进入项目目录
cd ros2_helper/src

# 运行扫描
python3 -m ros2_helper.main --ros2-bin /opt/ros/jazzy/bin/ros2
```

## 命令行选项

| 选项 | 说明 |
|------|------|
| `--ros2-bin PATH` | ROS2可执行文件路径 |
| `-m, --monitor` | 实时监控模式 |
| `-d, --duration N` | 监控持续时间(秒) |
| `-l, --list-saved` | 列出已保存的扫描结果 |
| `--create-config` | 创建默认配置文件 |

## 配置文件

配置文件默认路径: `~/.ros2_helper/config.yaml`

```yaml
ros2:
  distro: jazzy           # ROS2发行版 (jazzy, humble, foxy等)
  base_path: /opt/ros    # ROS2安装根目录
  ros2_bin: /opt/ros/jazzy/bin/ros2

scan:
  timeout: 5             # 扫描超时(秒)
  msg_parse_timeout: 5    # 消息解析超时(秒)
  refresh_interval: 2     # 自动刷新间隔(秒)
  strict_mode: false      # 严格模式

output:
  default_format: both    # 默认保存格式
  save_dir: ~/ros2_scans  # 保存目录
```

## 使用示例

```bash
# 基本扫描
python3 -m ros2_helper.main --ros2-bin /opt/ros/jazzy/bin/ros2

# 实时监控模式 (每2秒刷新)
python3 -m ros2_helper.main --ros2-bin /opt/ros/jazzy/bin/ros2 -m

# 指定监控时长
python3 -m ros2_helper.main --ros2-bin /opt/ros/jazzy/bin/ros2 -m -d 60

# 切换到humble
python3 -m ros2_helper.main --ros2-bin /opt/ros/humble/bin/ros2
```

## 消息格式识别

工具会根据消息类型自动识别数据格式:

| 消息类型 | 格式标识 |
|---------|---------|
| sensor_msgs/Image, CompressedImage | image/compressed |
| sensor_msgs/Imu | imu |
| sensor_msgs/LaserScan, PointCloud2 | radar/lidar |
| nav_msgs/Odometry | odometry |
| geometry_msgs/Twist | twist |
| 自定义消息 | json/binary |

## 输出示例

```json
{
  "scan_time": "2026-03-08T00:36:05",
  "topic_count": 2,
  "topics": [
    {
      "name": "/scan",
      "type": "sensor_msgs/msg/LaserScan",
      "data_format": "radar/lidar",
      "publishers": [...],
      "subscribers": [...],
      "qos_compatible": true,
      "qos_warnings": []
    }
  ]
}
```

## 依赖

- Python 3.8+
- pyyaml
- pick
- prompt-toolkit
- ROS2 (jazzy/humble/foxy)

## 安装

```bash
pip install -r requirements.txt
```

或安装为系统命令:

```bash
pip install -e .
ros2-helper --ros2-bin /opt/ros/jazzy/bin/ros2
```

## 注意事项

1. 配置文件修改distro即可切换ROS2版本
2. 非tty环境会自动降级为数字输入选择
3. 消息解析需要对应的ROS2 msg包已安装
