##排队服务器实现细则

###整体结构分为：
- 客户端Client
- 登录服务器LoginServer
- 排队服务器QueueServer

----


###执行流程：
1. Client向LoginServer发送登录请求。
2. LoginServer查检当前服务器负载，判断是否允许登录，负载过高，返回QueueServer的IP/PORT，并与LoginServer断开连接。
3. Client连接QueueServer，并保持PING/PONG请求。

----

###服务器实现原理：
1. #####登录服务器内部机制实现原理：#####
  - 如果LoginServer出现高负载情况，内部设定多长时间后允许下一个客户端登录。
  - LoginServer内部设定最大连接人数(即服务器承载人数上限)，负载人数(即人数达到负载人数后开始排队)。
  - LoginServer与QueueServer保持长连接，并间隔一定时间(30s)同步一次当前LoginServer当前负载情况。
  

2. #####排队服务器内部机制实现原理：#####
  - Client连接QueueServer后，保持长连接，每15s同步当前服务器负载。
  -	Client---------PING------->>>QueueServer
  -	Client<<<------PONG----------QueueServer
  -	QueueServer内部维护一个正在排队的Client列表(FIFO)。
  -	QueueServer与LoginServer保持长连接,每15s同步当前服务器负载，如果LoginServer未达到承载人数上限，依据允许登录
  

----
####可优化空间：
1. 涉及多次验证，Client登录高负载LoginServer会收到重定向到QueueServer的返回，并与LoginServer断开，
		高负载过后重新登录LoginServer。(优化：不做登录认证)
2. 