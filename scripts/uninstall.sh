#!/bin/bash
#
# LookingGlass 卸载脚本
#
# 使用方法:
#   ./scripts/uninstall.sh master    # 卸载 Master
#   ./scripts/uninstall.sh agent     # 卸载 Agent
#   ./scripts/uninstall.sh all       # 卸载所有组件
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

# 卸载组件
uninstall_component() {
    local component=$1  # master or agent
    local install_dir="/opt/lookingglass/${component}"

    log_info "卸载 ${component}..."

    # 停止服务
    log_step "停止 ${component} 服务..."
    sudo supervisorctl stop "lookingglass-${component}" 2>/dev/null || true

    # 移除 Supervisor 配置
    log_step "移除 Supervisor 配置..."
    local supervisor_conf="/etc/supervisor/conf.d/lookingglass-${component}.conf"
    if [[ -f "$supervisor_conf" ]]; then
        sudo rm -f "$supervisor_conf"
        log_info "已删除: $supervisor_conf"
    fi

    # 询问是否保留数据
    echo ""
    read -p "是否保留日志和配置文件? (y/N): " keep_data
    keep_data=${keep_data:-N}

    if [[ $keep_data =~ ^[Yy]$ ]]; then
        # 备份配置和日志
        local backup_dir="/tmp/lookingglass-backup-$(date +%Y%m%d_%H%M%S)"
        log_step "备份数据到 ${backup_dir}..."
        mkdir -p "$backup_dir"

        if [[ -d "$install_dir" ]]; then
            sudo cp -r "${install_dir}/config.yaml" "$backup_dir/" 2>/dev/null || true
            sudo cp -r "${install_dir}/logs" "$backup_dir/" 2>/dev/null || true
            sudo cp -r "${install_dir}/.api_key" "$backup_dir/" 2>/dev/null || true
            sudo chown -R $USER:$USER "$backup_dir"
            log_info "数据已备份到: $backup_dir"
        fi
    fi

    # 删除安装目录
    log_step "删除安装目录..."
    if [[ -d "$install_dir" ]]; then
        sudo rm -rf "$install_dir"
        log_info "已删除: $install_dir"
    fi

    # 重载 Supervisor
    log_step "重载 Supervisor..."
    sudo supervisorctl reread 2>/dev/null || true
    sudo supervisorctl update 2>/dev/null || true

    log_info "${component} 卸载完成"
}

# 完全清理
full_cleanup() {
    log_warn "执行完全清理..."

    # 停止所有服务
    log_step "停止所有 LookingGlass 服务..."
    sudo supervisorctl stop lookingglass-master 2>/dev/null || true
    sudo supervisorctl stop lookingglass-agent 2>/dev/null || true

    # 删除 Supervisor 配置
    log_step "删除所有 Supervisor 配置..."
    sudo rm -f /etc/supervisor/conf.d/lookingglass-*.conf

    # 询问是否保留数据
    echo ""
    read -p "是否保留所有日志和配置文件? (y/N): " keep_data
    keep_data=${keep_data:-N}

    if [[ $keep_data =~ ^[Yy]$ ]]; then
        local backup_dir="/tmp/lookingglass-backup-$(date +%Y%m%d_%H%M%S)"
        log_step "备份所有数据到 ${backup_dir}..."
        mkdir -p "$backup_dir"

        if [[ -d "/opt/lookingglass" ]]; then
            sudo cp -r /opt/lookingglass "$backup_dir/" 2>/dev/null || true
            sudo chown -R $USER:$USER "$backup_dir"
            log_info "数据已备份到: $backup_dir"
        fi
    fi

    # 删除所有文件
    log_step "删除所有安装文件..."
    sudo rm -rf /opt/lookingglass

    # 询问是否删除用户
    echo ""
    read -p "是否删除 lookingglass 用户? (y/N): " delete_user
    delete_user=${delete_user:-N}

    if [[ $delete_user =~ ^[Yy]$ ]]; then
        log_step "删除用户..."
        sudo userdel lookingglass 2>/dev/null || true
        log_info "用户已删除"
    fi

    # 重载 Supervisor
    log_step "重载 Supervisor..."
    sudo supervisorctl reread 2>/dev/null || true
    sudo supervisorctl update 2>/dev/null || true

    log_info "完全清理完成"
}

# 显示使用说明
show_usage() {
    cat <<EOF
LookingGlass 卸载脚本

使用方法:
  $0 [command]

命令:
  master      卸载 Master 服务
  agent       卸载 Agent 服务
  all         卸载所有组件（保留用户）
  purge       完全清理（包括用户）
  help        显示此帮助信息

示例:
  # 卸载 Master
  $0 master

  # 卸载 Agent
  $0 agent

  # 卸载所有组件
  $0 all

  # 完全清理（包括用户和所有数据）
  $0 purge

EOF
}

# 主函数
main() {
    local command=${1:-help}

    # 检查是否为 root
    if [[ $EUID -ne 0 ]] && ! sudo -n true 2>/dev/null; then
        log_error "此脚本需要 sudo 权限"
        exit 1
    fi

    case $command in
        master)
            echo ""
            log_info "=== 卸载 LookingGlass Master ==="
            echo ""
            uninstall_component "master"
            echo ""
            log_info "Master 已卸载"
            echo ""
            ;;

        agent)
            echo ""
            log_info "=== 卸载 LookingGlass Agent ==="
            echo ""
            uninstall_component "agent"
            echo ""
            log_info "Agent 已卸载"
            echo ""
            ;;

        all)
            echo ""
            log_info "=== 卸载所有 LookingGlass 组件 ==="
            echo ""
            uninstall_component "master"
            echo ""
            uninstall_component "agent"
            echo ""
            log_info "所有组件已卸载"
            echo ""
            ;;

        purge)
            echo ""
            log_warn "=== 完全清理 LookingGlass ==="
            echo ""
            read -p "这将删除所有 LookingGlass 相关文件，是否继续? (y/N): " confirm
            confirm=${confirm:-N}

            if [[ $confirm =~ ^[Yy]$ ]]; then
                full_cleanup
                echo ""
                log_info "完全清理完成"
            else
                log_info "已取消"
            fi
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
