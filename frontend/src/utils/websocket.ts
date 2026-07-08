export interface LogPayload {
  task: string;
  message: string;
}

export type StatusCallback = (payload: any) => void;
export type LogCallback = (payload: LogPayload) => void;
export type SysLogCallback = (line: string) => void;

class WsService {
  private ws: WebSocket | null = null;
  private listeners = new Set<StatusCallback>();
  private logListeners = new Map<string, Set<LogCallback>>();
  private sysLogListeners = new Set<SysLogCallback>();
  private reconnectAttempts = 0;
  private maxReconnectDelay = 30000;
  private pingInterval: any = null;
  private pongTimeout: any = null;
  public isConnected = false;
  private cachedStatus: any = null; // 缓存最新 status，供页面切换时立即回调

  connect(): void {
    if (this.ws && (this.ws.readyState === WebSocket.OPEN || this.ws.readyState === WebSocket.CONNECTING)) {
      return;
    }

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host;
    const wsUrl = `${protocol}//${host}/app/docker-updater/api/ws`;

    console.log(`[WS] 正在连接: ${wsUrl}`);
    this.ws = new WebSocket(wsUrl);

    this.ws.onopen = () => {
      console.log('[WS] 连接成功已建立');
      this.isConnected = true;
      this.reconnectAttempts = 0;
      this.startHeartbeat();

      // 重新订阅当前存活的全部日志信道，防止重连后丢失日志数据
      for (const container of this.logListeners.keys()) {
        this.send({ type: 'subscribe', target: `logs:${container}` });
      }
    };

    this.ws.onmessage = (event: MessageEvent) => {
      try {
        const data = JSON.parse(event.data);
        this.handleMessagePayload(data);
      } catch (err) {
        const lines = event.data.split('\n');
        if (lines.length > 1) {
          for (const line of lines) {
            if (line.trim()) {
              try {
                const data = JSON.parse(line);
                this.handleMessagePayload(data);
              } catch (e) {}
            }
          }
        }
      }
    };

    this.ws.onerror = (err) => {
      console.error('[WS] 发生异常: ', err);
    };

    this.ws.onclose = () => {
      console.log('[WS] 连接已关闭，准备尝试重连');
      this.isConnected = false;
      this.stopHeartbeat();
      this.reconnect();
    };
  }

  private handleMessagePayload(data: any): void {
    if (data.type === 'status') {
      this.cachedStatus = data.payload; // 实时更新缓存
      for (const listener of this.listeners) {
        listener(data.payload);
      }
    } else if (data.type === 'log') {
      const { container, task, message } = data.payload;
      const containerSet = this.logListeners.get(container);
      if (containerSet) {
        for (const listener of containerSet) {
          listener({ task, message });
        }
      }
    } else if (data.type === 'syslog') {
      const line: string = data.payload?.line ?? '';
      for (const listener of this.sysLogListeners) {
        listener(line);
      }
    } else if (data.type === 'pong') {
      clearTimeout(this.pongTimeout); // 收到 pong 才取消超时计时器
    }
  }

  private reconnect(): void {
    this.reconnectAttempts++;
    const delay = Math.min(
      Math.pow(2, this.reconnectAttempts) * 1000 + Math.random() * 1000,
      this.maxReconnectDelay
    );
    console.log(`[WS] 将在 ${Math.round(delay / 1000)} 秒后尝试第 ${this.reconnectAttempts} 次重连...`);
    setTimeout(() => {
      this.connect();
    }, delay);
  }

  send(msg: any): void {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(msg));
    }
  }

  private startHeartbeat(): void {
    this.pingInterval = setInterval(() => {
      this.send({ type: 'ping' });
      this.resetPongTimeout();
    }, 30000);
  }

  private stopHeartbeat(): void {
    clearInterval(this.pingInterval);
    clearTimeout(this.pongTimeout);
  }

  private resetPongTimeout(): void {
    clearTimeout(this.pongTimeout);
    this.pongTimeout = setTimeout(() => {
      console.warn('[WS] 心跳检测响应超时，强制重连');
      if (this.ws) {
        this.ws.close();
      }
    }, 10000);
  }

  // 注册全局状态更新回调，返回取消订阅函数
  // 若缓存存在则立即同步回调，消除页面切换时 loading 永久等待
  subscribeStatus(callback: StatusCallback): () => void {
    this.listeners.add(callback);
    this.connect();
    if (this.cachedStatus !== null) {
      callback(this.cachedStatus);
    }
    return () => {
      this.listeners.delete(callback);
    };
  }

  // 注册特定容器升级/回滚的流式日志回调，返回取消订阅函数
  subscribeLogs(containerName: string, callback: LogCallback): () => void {
    let containerSet = this.logListeners.get(containerName);
    if (!containerSet) {
      containerSet = new Set<LogCallback>();
      this.logListeners.set(containerName, containerSet);
      this.send({ type: 'subscribe', target: `logs:${containerName}` });
    }
    containerSet.add(callback);

    return () => {
      if (containerSet) {
        containerSet.delete(callback);
        if (containerSet.size === 0) {
          this.logListeners.delete(containerName);
          this.send({ type: 'unsubscribe', target: `logs:${containerName}` });
        }
      }
    };
  }
  // 订阅系统运行日志实时广播，返回取消订阅函数
  subscribeSysLog(callback: SysLogCallback): () => void {
    this.sysLogListeners.add(callback);
    this.connect();
    return () => {
      this.sysLogListeners.delete(callback);
    };
  }
}

export default new WsService();
