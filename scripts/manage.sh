#!/bin/bash
#
# LookingGlass 管理脚本
# 用于日常维护和管理 Master 和 Agent 服务
#
# 使用方法:
#   ./scripts/manage.sh status         # 查看所有服务状态
#   ./scripts/manage.sh start master   # 启动 Master
#   ./scripts/manage.sh logs agent     # 查看 Agent 日志
#

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
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

# 检查服务是否存在
check_service_exists() {
    local component=$1
    if ! sudo supervisorctl status "lookingglass-${component}" &>/dev/null; then
        return 1
    fi
    return 0
}

# 显示状态
show_status() {
    echo ""
    echo "=========================================="
    echo -e "${CYAN}LookingGlass 服务状态${NC}"
    echo "=========================================="
    echo ""

    # Master 状态
    if check_service_exists "master"; then
        echo -e "${BLUE}[Master]${NC}"
        sudo supervisorctl status lookingglass-master | while read line; do
            if echo "$line" | grep -q "RUNNING"; then
                echo -e "  ${GREEN}●${NC} $line"
            elif echo "$line" | grep -q "STOPPED"; then
                echo -e "  ${RED}●${NC} $line"
            else
                echo -e "  ${YELLOW}●${NC} $line"
            fi
        done
        echo ""

        # 显示端口占用
        if sudo supervisorctl status lookingglass-master | grep -q "RUNNING"; then
            echo "  端口监听:"
            sudo netstat -tlnp 2>/dev/null | grep -E ":(50051|8080)" | awk '{print "    "$4" -> "$7}' || \
            sudo ss -tlnp 2>/dev/null | grep -E ":(50051|8080)" | awk '{print "    "$4" -> "$6}' || \
            echo "    无法获取端口信息"
            echo ""
        fi
    else
        echo -e "${BLUE}[Master]${NC}"
        echo -e "  ${RED}●${NC} 未安装"
        echo ""
    fi

    # Agent 状态
    if check_service_exists "agent"; then
        echo -e "${BLUE}[Agent]${NC}"
        sudo supervisorctl status lookingglass-agent | while read line; do
            if echo "$line" | grep -q "RUNNING"; then
                echo -e "  ${GREEN}●${NC} $line"
            elif echo "$line" | grep -q "STOPPED"; then
                echo -e "  ${RED}●${NC} $line"
            else
                echo -e "  ${YELLOW}●${NC} $line"
            fi
        done
        echo ""
    else
        echo -e "${BLUE}[Agent]${NC}"
        echo -e "  ${RED}●${NC} 未安装"
        echo ""
    fi

    echo "=========================================="
    echo ""
}

# 启动服务
start_service() {
    local component=$1

    if [[ -z "$component" ]]; then
        log_info "启动所有服务..."
        sudo supervisorctl start lookingglass-master 2>/dev/null || true
        sudo supervisorctl start lookingglass-agent 2>/dev/null || true
    else
        if ! check_service_exists "$component"; then
            log_error "${component} 未安装"
            exit 1
        fi

        log_info "启动 ${component}..."
        sudo supervisorctl start "lookingglass-${component}"
    fi

    sleep 1
    show_status
}

# 停止服务
stop_service() {
    local component=$1

    if [[ -z "$component" ]]; then
        log_info "停止所有服务..."
        sudo supervisorctl stop lookingglass-master 2>/dev/null || true
        sudo supervisorctl stop lookingglass-agent 2>/dev/null || true
    else
        if ! check_service_exists "$component"; then
            log_error "${component} 未安装"
            exit 1
        fi

        log_info "停止 ${component}..."
        sudo supervisorctl stop "lookingglass-${component}"
    fi

    sleep 1
    show_status
}

# 重启服务
restart_service() {
    local component=$1

    if [[ -z "$component" ]]; then
        log_info "重启所有服务..."
        sudo supervisorctl restart lookingglass-master 2>/dev/null || true
        sudo supervisorctl restart lookingglass-agent 2>/dev/null || true
    else
        if ! check_service_exists "$component"; then
            log_error "${component} 未安装"
            exit 1
        fi

        log_info "重启 ${component}..."
        sudo supervisorctl restart "lookingglass-${component}"
    fi

    sleep 1
    show_status
}

# 查看日志
view_logs() {
    local component=$1
    local follow=${2:-false}

    if [[ -z "$component" ]]; then
        log_error "请指定组件: master 或 agent"
        exit 1
    fi

    if ! check_service_exists "$component"; then
        log_error "${component} 未安装"
        exit 1
    fi

    if [[ $follow == true ]]; then
        log_info "实时查看 ${component} 日志 (Ctrl+C 退出)..."
        echo ""
        sudo supervisorctl tail -f "lookingglass-${component}" stdout
    else
        log_info "查看 ${component} 最近日志..."
        echo ""
        sudo supervisorctl tail "lookingglass-${component}" stdout 50
    fi
}

# 查看错误日志
view_error_logs() {
    local component=$1
    local follow=${2:-false}

    if [[ -z "$component" ]]; then
        log_error "请指定组件: master 或 agent"
        exit 1
    fi

    if ! check_service_exists "$component"; then
        log_error "${component} 未安装"
        exit 1
    fi

    if [[ $follow == true ]]; then
        log_info "实时查看 ${component} 错误日志 (Ctrl+C 退出)..."
        echo ""
        sudo supervisorctl tail -f "lookingglass-${component}" stderr
    else
        log_info "查看 ${component} 最近错误日志..."
        echo ""
        sudo supervisorctl tail "lookingglass-${component}" stderr 50
    fi
}

# 查看配置
view_config() {
    local component=$1

    if [[ -z "$component" ]]; then
        log_error "请指定组件: master 或 agent"
        exit 1
    fi

    local config_file="/opt/lookingglass/${component}/config.yaml"

    if [[ ! -f "$config_file" ]]; then
        log_error "配置文件不存在: $config_file"
        exit 1
    fi

    log_info "配置文件: $config_file"
    echo ""
    cat "$config_file"
}

# 编辑配置
edit_config() {
    local component=$1

    if [[ -z "$component" ]]; then
        log_error "请指定组件: master 或 agent"
        exit 1
    fi

    local config_file="/opt/lookingglass/${component}/config.yaml"

    if [[ ! -f "$config_file" ]]; then
        log_error "配置文件不存在: $config_file"
        exit 1
    fi

    local editor=${EDITOR:-vi}
    log_info "编辑配置文件: $config_file"
    sudo $editor "$config_file"

    echo ""
    read -p "是否重启服务以应用配置? (Y/n): " restart_confirm
    restart_confirm=${restart_confirm:-Y}

    if [[ $restart_confirm =~ ^[Yy]$ ]]; then
        restart_service "$component"
    fi
}

# 显示 API Key
show_api_key() {
    local api_key_file="/opt/lookingglass/master/.api_key"

    if [[ ! -f "$api_key_file" ]]; then
        log_error "API Key 文件不存在，Master 可能未安装"
        exit 1
    fi

    echo ""
    echo "=========================================="
    echo -e "${CYAN}Master API Key${NC}"
    echo "=========================================="
    echo ""
    sudo cat "$api_key_file"
    echo ""
    echo "=========================================="
    echo ""
    echo "使用此 API Key 配置 Agent 连接到 Master"
    echo ""
}

# 健康检查
health_check() {
    echo ""
    echo "=========================================="
    echo -e "${CYAN}LookingGlass 健康检查${NC}"
    echo "=========================================="
    echo ""

    local all_healthy=true

    # 检查 Master
    if check_service_exists "master"; then
        echo -e "${BLUE}[Master]${NC}"

        # 检查进程状态
        if sudo supervisorctl status lookingglass-master | grep -q "RUNNING"; then
            echo -e "  ${GREEN}✓${NC} 进程运行中"

            # 检查端口
            if sudo netstat -tlnp 2>/dev/null | grep -q ":8080" || sudo ss -tlnp 2>/dev/null | grep -q ":8080"; then
                echo -e "  ${GREEN}✓${NC} Web 端口 (8080) 监听正常"

                # 尝试访问 API
                if curl -s --max-time 2 http://localhost:8080/api/agents >/dev/null 2>&1; then
                    echo -e "  ${GREEN}✓${NC} API 响应正常"
                else
                    echo -e "  ${YELLOW}⚠${NC} API 无响应"
                    all_healthy=false
                fi
            else
                echo -e "  ${RED}✗${NC} Web 端口 (8080) 未监听"
                all_healthy=false
            fi

            if sudo netstat -tlnp 2>/dev/null | grep -q ":50051" || sudo ss -tlnp 2>/dev/null | grep -q ":50051"; then
                echo -e "  ${GREEN}✓${NC} gRPC 端口 (50051) 监听正常"
            else
                echo -e "  ${RED}✗${NC} gRPC 端口 (50051) 未监听"
                all_healthy=false
            fi
        else
            echo -e "  ${RED}✗${NC} 进程未运行"
            all_healthy=false
        fi
        echo ""
    fi

    # 检查 Agent
    if check_service_exists "agent"; then
        echo -e "${BLUE}[Agent]${NC}"

        if sudo supervisorctl status lookingglass-agent | grep -q "RUNNING"; then
            echo -e "  ${GREEN}✓${NC} 进程运行中"

            # 检查是否能连接到 Master
            local master_host=$(sudo grep "host:" /opt/lookingglass/agent/config.yaml | awk '{print $2}' | tr -d '"')
            if [[ -n "$master_host" ]]; then
                echo "  Master 地址: $master_host"

                local master_ip=$(echo "$master_host" | cut -d: -f1)
                local master_port=$(echo "$master_host" | cut -d: -f2)

                if timeout 2 bash -c "cat < /dev/null > /dev/tcp/${master_ip}/${master_port}" 2>/dev/null; then
                    echo -e "  ${GREEN}✓${NC} 可以连接到 Master"
                else
                    echo -e "  ${YELLOW}⚠${NC} 无法连接到 Master"
                    all_healthy=false
                fi
            fi
        else
            echo -e "  ${RED}✗${NC} 进程未运行"
            all_healthy=false
        fi
        echo ""
    fi

    echo "=========================================="
    echo ""

    if [[ $all_healthy == true ]]; then
        echo -e "${GREEN}✓ 所有检查通过${NC}"
    else
        echo -e "${YELLOW}⚠ 部分检查失败，请查看详细信息${NC}"
    fi
    echo ""
}

# 显示使用说明
show_usage() {
    cat <<EOF
LookingGlass 管理脚本

使用方法:
  $0 [command] [component]

命令:
  status              查看所有服务状态
  start [component]   启动服务 (master/agent/all)
  stop [component]    停止服务 (master/agent/all)
  restart [component] 重启服务 (master/agent/all)
  logs <component>    查看日志 (master/agent)
  logs-f <component>  实时查看日志 (master/agent)
  error <component>   查看错误日志 (master/agent)
  error-f <component> 实时查看错误日志 (master/agent)
  config <component>  查看配置 (master/agent)
  edit <component>    编辑配置 (master/agent)
  apikey              显示 Master API Key
  health              健康检查
  help                显示此帮助信息

示例:
  # 查看状态
  $0 status

  # 启动 Master
  $0 start master

  # 重启所有服务
  $0 restart

  # 查看 Agent 日志
  $0 logs agent

  # 实时查看 Master 日志
  $0 logs-f master

  # 编辑 Agent 配置
  $0 edit agent

  # 健康检查
  $0 health

  # 显示 API Key
  $0 apikey

EOF
}

# 主函数
main() {
    local command=${1:-status}
    local component=${2:-}

    # 检查是否安装了 supervisorctl
    if ! command -v supervisorctl &> /dev/null; then
        log_error "未找到 supervisorctl，请先安装 supervisor"
        exit 1
    fi

    case $command in
        status)
            show_status
            ;;

        start)
            start_service "$component"
            ;;

        stop)
            stop_service "$component"
            ;;

        restart)
            restart_service "$component"
            ;;

        logs)
            view_logs "$component" false
            ;;

        logs-f)
            view_logs "$component" true
            ;;

        error)
            view_error_logs "$component" false
            ;;

        error-f)
            view_error_logs "$component" true
            ;;

        config)
            view_config "$component"
            ;;

        edit)
            edit_config "$component"
            ;;

        apikey|api-key)
            show_api_key
            ;;

        health|check)
            health_check
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
