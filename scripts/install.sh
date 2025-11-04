#!/bin/bash
#
# LookingGlass 自动安装脚本
# 用于在不依赖 Docker 的情况下部署 Master 和 Agent
#
# 使用方法:
#   ./scripts/install.sh master    # 仅安装 Master
#   ./scripts/install.sh agent     # 仅安装 Agent
#   ./scripts/install.sh all       # 安装 Master 和 Agent
#

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 日志函数
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_step() {
    echo -e "${BLUE}[STEP]${NC} $1"
}

# 生成随机字符串
generate_random_string() {
    local length=${1:-32}
    openssl rand -hex $((length / 2)) 2>/dev/null || cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w $length | head -n 1
}

# 生成随机 Agent ID
generate_agent_id() {
    echo "agent-$(generate_random_string 8)"
}

# 检测公网 IP
detect_public_ip() {
    local ip=""
    # 尝试多个 API
    for api in "https://api.ipify.org" "https://ifconfig.me/ip" "https://icanhazip.com" "https://ident.me"; do
        ip=$(curl -s --max-time 5 "$api" 2>/dev/null | tr -d '\n')
        if [[ $ip =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
            echo "$ip"
            return 0
        fi
    done

    # 如果失败，返回本地 IP
    log_warn "无法检测公网 IP，使用本地 IP"
    ip=$(hostname -I | awk '{print $1}')
    echo "$ip"
}

# 检测系统类型
detect_os() {
    if [[ -f /etc/os-release ]]; then
        . /etc/os-release
        echo "$ID"
    else
        uname -s | tr '[:upper:]' '[:lower:]'
    fi
}

# 检测包管理器
detect_package_manager() {
    if command -v apt-get &> /dev/null; then
        echo "apt"
    elif command -v yum &> /dev/null; then
        echo "yum"
    elif command -v dnf &> /dev/null; then
        echo "dnf"
    elif command -v apk &> /dev/null; then
        echo "apk"
    else
        echo "unknown"
    fi
}

# 安装依赖工具
install_dependencies() {
    log_step "安装系统依赖..."

    local pkg_mgr=$(detect_package_manager)
    local tools="supervisor curl wget"

    case $pkg_mgr in
        apt)
            sudo apt-get update -qq
            sudo apt-get install -y -qq supervisor curl wget >/dev/null 2>&1
            ;;
        yum|dnf)
            sudo $pkg_mgr install -y -q supervisor curl wget >/dev/null 2>&1
            ;;
        apk)
            sudo apk add --no-cache supervisor curl wget >/dev/null 2>&1
            ;;
        *)
            log_error "不支持的包管理器，请手动安装: supervisor, curl, wget"
            exit 1
            ;;
    esac

    log_info "系统依赖安装完成"
}

# 安装诊断工具（仅 Agent 需要）
install_diagnostic_tools() {
    log_step "安装网络诊断工具..."

    local pkg_mgr=$(detect_package_manager)

    case $pkg_mgr in
        apt)
            sudo apt-get install -y -qq iputils-ping mtr-tiny >/dev/null 2>&1
            ;;
        yum|dnf)
            sudo $pkg_mgr install -y -q iputils mtr >/dev/null 2>&1
            ;;
        apk)
            sudo apk add --no-cache iputils mtr >/dev/null 2>&1
            ;;
        *)
            log_warn "无法自动安装诊断工具，请手动安装: ping, mtr"
            ;;
    esac

    # 安装 NextTrace
    if ! command -v nexttrace &> /dev/null; then
        log_info "安装 NextTrace..."
        local arch=$(uname -m)
        local nexttrace_arch="amd64"

        case $arch in
            x86_64)
                nexttrace_arch="amd64"
                ;;
            aarch64|arm64)
                nexttrace_arch="arm64"
                ;;
            *)
                log_warn "不支持的架构 $arch，跳过 NextTrace 安装"
                return
                ;;
        esac

        wget -q -O /tmp/nexttrace "https://github.com/nxtrace/NTrace-core/releases/latest/download/nexttrace_linux_${nexttrace_arch}" 2>/dev/null || {
            log_warn "下载 NextTrace 失败，跳过"
            return
        }

        sudo mv /tmp/nexttrace /usr/local/bin/nexttrace
        sudo chmod +x /usr/local/bin/nexttrace
        log_info "NextTrace 安装完成"
    else
        log_info "NextTrace 已安装"
    fi
}

# 创建用户
create_user() {
    if ! id lookingglass &>/dev/null; then
        log_step "创建 lookingglass 用户..."
        sudo useradd -r -s /bin/false -d /opt/lookingglass lookingglass 2>/dev/null || true
        log_info "用户创建完成"
    else
        log_info "用户 lookingglass 已存在"
    fi
}

# 生成 Master 配置文件
generate_master_config() {
    local install_dir=$1
    local api_key=$2

    log_step "生成 Master 配置文件..."

    cat > "${install_dir}/config.yaml" <<EOF
# LookingGlass Master Configuration
# Auto-generated on $(date)

server:
  grpc_port: 50051        # gRPC server port for agent connections
  ws_port: 8080          # WebSocket port for frontend
  http_enabled: true     # Enable HTTP server for static files

auth:
  mode: api_key          # Authentication mode: api_key or ip_whitelist
  api_keys:
    - "${api_key}"       # API key for agent authentication

concurrency:
  global_max: 100        # Maximum concurrent tasks across all agents
  agent_default_max: 10  # Default max concurrent tasks per agent
  agent_limits: {}       # Per-agent limits: agent_id: max_tasks

heartbeat:
  interval: 30           # Heartbeat interval in seconds
  timeout: 90            # Heartbeat timeout in seconds

log:
  level: info            # Log level: debug, info, warn, error
  file: logs/master.log
  console: true

# Web frontend settings
web:
  static_dir: web        # Static files directory

# Branding customization (optional)
branding:
  site_title: "LookingGlass"
  logo_text: "LG"
  subtitle: "Network Diagnostic Platform"
  footer_text: "Powered by LookingGlass"
EOF

    log_info "Master 配置文件已生成: ${install_dir}/config.yaml"
}

# 生成 Agent 配置文件
generate_agent_config() {
    local install_dir=$1
    local api_key=$2
    local master_host=$3
    local agent_id=$4
    local public_ip=$5

    log_step "生成 Agent 配置文件..."

    cat > "${install_dir}/config.yaml" <<EOF
# LookingGlass Agent Configuration
# Auto-generated on $(date)

agent:
  id: "${agent_id}"                    # Unique agent identifier
  name: "Auto-Agent-${agent_id}"      # Display name
  ipv4: "${public_ip}"                # Public IPv4 address (leave empty for auto-detection)
  ipv6: ""                            # Public IPv6 address (leave empty for auto-detection)
  hide_ip: true                       # Mask last 2 octets of IP address
  max_concurrent: 10                  # Maximum concurrent tasks

  # Agent metadata
  metadata:
    location: "$(hostname)"           # Geographic location
    provider: "Self-Hosted"           # Service provider
    idc: "$(hostname)"                # Datacenter
    description: "Auto-deployed agent"

master:
  host: "${master_host}"              # Master server address (host:port)
  api_key: "${api_key}"               # API key for authentication
  tls_enabled: false                  # Enable TLS for gRPC connection
  heartbeat_interval: 30              # Heartbeat interval in seconds
  reconnect:
    initial_interval: 1               # Initial reconnect interval (seconds)
    max_interval: 60                  # Maximum reconnect interval (seconds)
    multiplier: 2                     # Backoff multiplier

executor:
  global_concurrency: 10              # Global concurrency limit

  tasks:
    # Built-in tasks
    ping:
      enabled: true
      display_name: "Ping"
      requires_target: true
      concurrency:
        max: 5

    mtr:
      enabled: true
      display_name: "MTR"
      requires_target: true
      concurrency:
        max: 3

    nexttrace:
      enabled: true
      display_name: "NextTrace"
      requires_target: true
      concurrency:
        max: 2

    # Custom command example (disabled by default)
    # curl_test:
    #   enabled: false
    #   display_name: "HTTP Check"
    #   requires_target: true
    #   executor:
    #     type: command
    #     path: "/usr/bin/curl"
    #     default_args: ["-I", "-m", "10", "{target}"]
    #   concurrency:
    #     max: 2

log:
  level: info                         # Log level: debug, info, warn, error
  file: logs/agent.log
  console: true
EOF

    log_info "Agent 配置文件已生成: ${install_dir}/config.yaml"
}

# 生成 Supervisor 配置
generate_supervisor_config() {
    local component=$1  # master or agent
    local install_dir=$2
    local binary_path="${install_dir}/${component}"
    local config_path="${install_dir}/config.yaml"

    log_step "生成 Supervisor 配置..."

    local supervisor_conf="/etc/supervisor/conf.d/lookingglass-${component}.conf"

    sudo tee "$supervisor_conf" > /dev/null <<EOF
[program:lookingglass-${component}]
command=${binary_path} -config ${config_path}
directory=${install_dir}
user=lookingglass
autostart=true
autorestart=true
startretries=3
stderr_logfile=${install_dir}/logs/${component}-error.log
stdout_logfile=${install_dir}/logs/${component}-output.log
stdout_logfile_maxbytes=10MB
stderr_logfile_maxbytes=10MB
stdout_logfile_backups=5
stderr_logfile_backups=5
priority=1
EOF

    log_info "Supervisor 配置已生成: $supervisor_conf"
}

# 安装 Master
install_master() {
    log_info "开始安装 Master..."

    local install_dir="/opt/lookingglass/master"
    local api_key=$(generate_random_string 32)

    # 创建目录
    log_step "创建安装目录..."
    sudo mkdir -p "$install_dir"/{logs,web}

    # 复制二进制文件
    if [[ ! -f "bin/master" ]]; then
        log_error "未找到 master 二进制文件，请先运行 'make build'"
        exit 1
    fi

    log_step "安装 Master 二进制文件..."
    sudo cp bin/master "$install_dir/"
    sudo chmod +x "${install_dir}/master"

    # 复制 web 文件
    if [[ -d "web" ]]; then
        log_step "安装 Web 文件..."
        sudo cp -r web/* "${install_dir}/web/"
    fi

    # 生成配置
    generate_master_config "$install_dir" "$api_key"

    # 设置权限
    sudo chown -R lookingglass:lookingglass "$install_dir"

    # 生成 Supervisor 配置
    generate_supervisor_config "master" "$install_dir"

    # 保存 API Key
    echo "$api_key" | sudo tee "${install_dir}/.api_key" > /dev/null
    sudo chmod 600 "${install_dir}/.api_key"
    sudo chown lookingglass:lookingglass "${install_dir}/.api_key"

    log_info "Master 安装完成！"
    echo ""
    echo "=========================================="
    echo -e "${GREEN}Master 安装信息${NC}"
    echo "=========================================="
    echo "安装目录: $install_dir"
    echo "配置文件: ${install_dir}/config.yaml"
    echo "API Key:  $api_key"
    echo "gRPC 端口: 50051"
    echo "Web 端口:  8080"
    echo ""
    echo -e "${YELLOW}重要：请保存 API Key，Agent 连接需要使用此 Key${NC}"
    echo "=========================================="
    echo ""
}

# 安装 Agent
install_agent() {
    log_info "开始安装 Agent..."

    local install_dir="/opt/lookingglass/agent"
    local agent_id=$(generate_agent_id)
    local public_ip=$(detect_public_ip)

    # 询问 Master 地址和 API Key
    echo ""
    read -p "请输入 Master 地址 (例如: 192.168.1.100:50051): " master_host
    if [[ -z "$master_host" ]]; then
        log_error "Master 地址不能为空"
        exit 1
    fi

    read -p "请输入 Master API Key: " api_key
    if [[ -z "$api_key" ]]; then
        log_error "API Key 不能为空"
        exit 1
    fi

    # 创建目录
    log_step "创建安装目录..."
    sudo mkdir -p "$install_dir"/logs

    # 复制二进制文件
    if [[ ! -f "bin/agent" ]]; then
        log_error "未找到 agent 二进制文件，请先运行 'make build'"
        exit 1
    fi

    log_step "安装 Agent 二进制文件..."
    sudo cp bin/agent "$install_dir/"
    sudo chmod +x "${install_dir}/agent"

    # 生成配置
    generate_agent_config "$install_dir" "$api_key" "$master_host" "$agent_id" "$public_ip"

    # 设置权限
    sudo chown -R lookingglass:lookingglass "$install_dir"

    # 生成 Supervisor 配置
    generate_supervisor_config "agent" "$install_dir"

    log_info "Agent 安装完成！"
    echo ""
    echo "=========================================="
    echo -e "${GREEN}Agent 安装信息${NC}"
    echo "=========================================="
    echo "安装目录:   $install_dir"
    echo "配置文件:   ${install_dir}/config.yaml"
    echo "Agent ID:   $agent_id"
    echo "公网 IP:    $public_ip"
    echo "Master 地址: $master_host"
    echo "=========================================="
    echo ""
}

# 重载 Supervisor
reload_supervisor() {
    log_step "重载 Supervisor 配置..."

    if command -v supervisorctl &> /dev/null; then
        sudo supervisorctl reread
        sudo supervisorctl update
        log_info "Supervisor 配置已重载"
    else
        log_warn "未找到 supervisorctl，请手动重载配置"
    fi
}

# 启动服务
start_services() {
    local component=$1

    log_step "启动 ${component} 服务..."

    sudo supervisorctl start "lookingglass-${component}" 2>/dev/null || {
        log_warn "启动失败，尝试重启 Supervisor..."
        sudo systemctl restart supervisor 2>/dev/null || sudo service supervisor restart 2>/dev/null
        sleep 2
        sudo supervisorctl start "lookingglass-${component}"
    }

    sleep 2

    # 检查状态
    if sudo supervisorctl status "lookingglass-${component}" | grep -q "RUNNING"; then
        log_info "${component} 服务启动成功"
    else
        log_error "${component} 服务启动失败，请检查日志"
        sudo supervisorctl status "lookingglass-${component}"
    fi
}

# 显示使用说明
show_usage() {
    cat <<EOF
LookingGlass 自动安装脚本

使用方法:
  $0 [command] [options]

命令:
  master      安装 Master 服务
  agent       安装 Agent 服务
  all         安装 Master 和 Agent 服务
  help        显示此帮助信息

选项:
  --skip-deps        跳过依赖安装
  --skip-tools       跳过诊断工具安装（仅 Agent）
  --no-start         不自动启动服务

示例:
  # 安装 Master
  $0 master

  # 安装 Agent
  $0 agent

  # 安装 Master 和 Agent
  $0 all

  # 跳过依赖安装
  $0 master --skip-deps

安装后管理:
  # 查看服务状态
  sudo supervisorctl status

  # 启动服务
  sudo supervisorctl start lookingglass-master
  sudo supervisorctl start lookingglass-agent

  # 停止服务
  sudo supervisorctl stop lookingglass-master
  sudo supervisorctl stop lookingglass-agent

  # 重启服务
  sudo supervisorctl restart lookingglass-master
  sudo supervisorctl restart lookingglass-agent

  # 查看日志
  sudo supervisorctl tail -f lookingglass-master stdout
  sudo supervisorctl tail -f lookingglass-agent stdout

文档:
  完整文档: https://github.com/lureiny/lookingglass
  配置指南: docs/TASK_CONFIG.md
  部署指南: docs/DEPLOYMENT.md

EOF
}

# 主函数
main() {
    local command=${1:-help}
    local skip_deps=false
    local skip_tools=false
    local no_start=false

    # 解析参数
    shift || true
    while [[ $# -gt 0 ]]; do
        case $1 in
            --skip-deps)
                skip_deps=true
                shift
                ;;
            --skip-tools)
                skip_tools=true
                shift
                ;;
            --no-start)
                no_start=true
                shift
                ;;
            *)
                log_error "未知选项: $1"
                show_usage
                exit 1
                ;;
        esac
    done

    # 检查是否为 root
    if [[ $EUID -ne 0 ]] && ! sudo -n true 2>/dev/null; then
        log_error "此脚本需要 sudo 权限，请确保当前用户有 sudo 权限"
        exit 1
    fi

    case $command in
        master)
            echo ""
            log_info "=== 安装 LookingGlass Master ==="
            echo ""

            if [[ $skip_deps == false ]]; then
                install_dependencies
            fi

            create_user
            install_master
            reload_supervisor

            if [[ $no_start == false ]]; then
                start_services "master"
            fi

            echo ""
            log_info "Master 安装完成！"
            echo ""
            echo "快速开始:"
            echo "  查看状态: sudo supervisorctl status lookingglass-master"
            echo "  查看日志: sudo supervisorctl tail -f lookingglass-master stdout"
            echo "  访问 Web: http://$(hostname -I | awk '{print $1}'):8080"
            echo ""
            ;;

        agent)
            echo ""
            log_info "=== 安装 LookingGlass Agent ==="
            echo ""

            if [[ $skip_deps == false ]]; then
                install_dependencies
            fi

            if [[ $skip_tools == false ]]; then
                install_diagnostic_tools
            fi

            create_user
            install_agent
            reload_supervisor

            if [[ $no_start == false ]]; then
                start_services "agent"
            fi

            echo ""
            log_info "Agent 安装完成！"
            echo ""
            echo "快速开始:"
            echo "  查看状态: sudo supervisorctl status lookingglass-agent"
            echo "  查看日志: sudo supervisorctl tail -f lookingglass-agent stdout"
            echo ""
            ;;

        all)
            echo ""
            log_info "=== 安装 LookingGlass Master 和 Agent ==="
            echo ""

            if [[ $skip_deps == false ]]; then
                install_dependencies
            fi

            if [[ $skip_tools == false ]]; then
                install_diagnostic_tools
            fi

            create_user
            install_master

            echo ""
            log_info "等待 3 秒后安装 Agent..."
            sleep 3

            # Agent 自动使用 localhost 的 Master
            local master_host="127.0.0.1:50051"
            local api_key=$(cat /opt/lookingglass/master/.api_key)
            local agent_id=$(generate_agent_id)
            local public_ip=$(detect_public_ip)
            local install_dir="/opt/lookingglass/agent"

            sudo mkdir -p "$install_dir"/logs
            sudo cp bin/agent "$install_dir/"
            sudo chmod +x "${install_dir}/agent"

            generate_agent_config "$install_dir" "$api_key" "$master_host" "$agent_id" "$public_ip"
            sudo chown -R lookingglass:lookingglass "$install_dir"
            generate_supervisor_config "agent" "$install_dir"

            reload_supervisor

            if [[ $no_start == false ]]; then
                start_services "master"
                sleep 2
                start_services "agent"
            fi

            echo ""
            log_info "全部安装完成！"
            echo ""
            echo "快速开始:"
            echo "  查看状态: sudo supervisorctl status"
            echo "  访问 Web: http://$(hostname -I | awk '{print $1}'):8080"
            echo ""
            ;;

        help|--help|-h)
            show_usage
            ;;

        *)
            log_error "未知命令: $command"
            show_usage
            exit 1
            ;;
    esac
}

# 运行主函数
main "$@"
