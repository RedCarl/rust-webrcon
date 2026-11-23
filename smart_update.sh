#!/bin/bash
# 更新模组下载文件
\cp -rf /data/mods_list.sh /app/lgsm/modules/

# 切换到 LinuxGSM 目录
cd /app

# 检查更新
./rustserver check-update > /tmp/check.log 2>&1

if ! grep -q "No update" /tmp/check.log 2>/dev/null; then
    echo "[$(date)] 检测到 Rust 更新，开始预警流程..." >> /data/update.log
    
    # 10分钟警告（通过 RCON 接口，仅提示，不直接执行重启）
    SERVER_ID=1
    RCON_PASSWORD=${WEBRCON_PASSWORD:-"RUST630669"}
    API_URL=${WEBRCON_API_URL:-"https://rust-webrcon.carljoy.cn"}
    
    echo "[$(date)] 发送 RCON 重启警告..." >> /data/update.log
    
    curl -X POST "$API_URL/api/servers/$SERVER_ID/command" \
      -H "Content-Type: application/json" \
      -d "{
        \"command\": \"say 游戏服务器检测到更新，将在 10 分钟后重启，请检查客户端是否需要更新\",
        \"password\": \"$RCON_PASSWORD\"
      }" || echo "[$(date)] RCON 发送失败" >> /data/update.log

    echo "[$(date)] 等待 300 秒后发送 5 分钟提醒..." >> /data/update.log
    sleep 300

    curl -X POST "$API_URL/api/servers/$SERVER_ID/command" \
      -H "Content-Type: application/json" \
      -d "{
        \"command\": \"say 游戏服务器将在 5 分钟后重启，请尽快回家并整理物资\",
        \"password\": \"$RCON_PASSWORD\"
      }" || echo "[$(date)] 5 分钟提醒发送失败" >> /data/update.log

    echo "[$(date)] 等待 240 秒后发送 1 分钟提醒..." >> /data/update.log
    sleep 240

    curl -X POST "$API_URL/api/servers/$SERVER_ID/command" \
      -H "Content-Type: application/json" \
      -d "{
        \"command\": \"say 游戏服务器将在 1 分钟后重启，正在准备保存世界数据\",
        \"password\": \"$RCON_PASSWORD\"
      }" || echo "[$(date)] 1 分钟提醒发送失败" >> /data/update.log

    echo "[$(date)] 等待 50 秒后执行强制保存..." >> /data/update.log
    sleep 50

    echo "[$(date)] 通过 RCON 执行 server.save ..." >> /data/update.log
    curl -X POST "$API_URL/api/servers/$SERVER_ID/command" \
      -H "Content-Type: application/json" \
      -d "{
        \"command\": \"server.save\",
        \"password\": \"$RCON_PASSWORD\"
      }" || echo "[$(date)] server.save 发送失败" >> /data/update.log

    echo "[$(date)] 等待 10 秒让世界数据落盘..." >> /data/update.log
    sleep 10

    # ========== 服务器更新循环（失败重试，无更新则跳过）==========
    echo "[$(date)] 开始执行服务器更新..." >> /data/update.log
    ./rustserver stop > /tmp/stop.log 2>&1
    
    while true; do
        ./rustserver update > /tmp/update.log 2>&1
        
        # 检查是否没有可用更新
        if grep -q "No update" /tmp/update.log 2>/dev/null; then
            echo "[$(date)] 服务器无可用更新，跳过" >> /data/update.log
            break
        fi
        
        # 检查是否更新成功
        if grep -q "complete" /tmp/update.log 2>/dev/null || grep -q "Success" /tmp/update.log 2>/dev/null; then
            echo "[$(date)] 服务器更新成功" >> /data/update.log
            break
        fi
        
        # 更新失败，等待重试
        echo "[$(date)] 服务器更新失败，5秒后重试..." >> /data/update.log
        sleep 5
    done

    # ========== 模组更新循环（失败重试，无更新则跳过）==========
    echo "[$(date)] 开始执行模组更新..." >> /data/update.log
    
    while true; do
        ./rustserver mu > /tmp/mods-update.log 2>&1

        # 检查是否没有安装模组
        if grep -q "No installed mods" /tmp/mods-update.log 2>/dev/null; then
            echo "rustoxide" | ./rustserver mi >> /data/update.log
        fi

        # 检查是否没有可用更新
        if grep -q "No update" /tmp/mods-update.log 2>/dev/null; then
            echo "[$(date)] 模组无可用更新，跳过" >> /data/update.log
            break
        fi
        
        # 检查是否更新成功
        if grep -q "complete" /tmp/mods-update.log 2>/dev/null || grep -q "Success" /tmp/mods-update.log 2>/dev/null; then
            echo "[$(date)] 模组更新成功" >> /data/update.log
            break
        fi
        
        # 更新失败，等待重试
        echo "[$(date)] 模组更新失败，5秒后重试..." >> /data/update.log
        sleep 5
    done
    
    # 启动服务器
    echo "[$(date)] 启动服务器..." >> /data/update.log
    ./rustserver start > /tmp/start.log 2>&1

    echo "[$(date)] 更新流程完成，服务器已启动" >> /data/update.log

else
    echo "[$(date)] 无可用更新" >> /data/update.log
fi
