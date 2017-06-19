JSONRPC
=======

JSONRPC это реализация протокола JSONRPC v2 для Go.

Этот пакет использует net/rpc, но вместо постоянных соединений использует 
один HTTP запрос на один вызов. Другие отличия:

- Несколько кодеков для сервера.
- Кодек выбирается в зависимости от "Content-Type" заголовка.
- Методы Service также получают http.Request в качестве параметра (опиционально).
- Этот пакет может быть использован на Google App Engine.

Настройка сервера и регистрация кодека и сервиса:
``` 
import (
    "http"
    
    "github.com/l-vitaly/jsonrpc"
    "github.com/l-vitaly/jsonrpc/json"
)

func init() {
    s := rpc.NewServer()   
    s.RegisterCodec(json2.NewCodec(), "application/json")
    s.RegisterService(new(HelloService), "")
    
    http.Handle("/rpc", s)
}
```

Этот сервер обрабатывает запросы для "/rpc" с использованием кодека JSON2.
Кодек привязан к типу контента. В приведенном выше примере, в формате JSON2 
зарегистрированный кодек для обслуживания запросов у которых заголовок 
"Content-Type" равен "application/json".

Сервис может быть зарегистрирован с использованием имени. 
Если имя пустое, как и в Примере выше, то имя будет взято из типа структуры.

Определим простой сервис:

```
type HelloReq struct {
    Who string
}

type HelloResp struct {
    Message string
}

type PingResp struct {
    Message string
}

type HelloService struct {}

func (h *HelloService) Say(r *http.Request, req *HelloReq) (interface{}, error) { 
    return HelloResp{Who: "Hello, " + req.Who + "!"}, nil
}

func (h *HelloService) Ping(r *http.Request) (interface{}, error) {
    return PingResp{Message: "Pong"}, nil
}
```

Приведенный выше пример определяет сервис с методами "HelloService.Say", "HelloService.Ping".

Медоты сервиса будут доступны если следуют этим правилам:

- Имя метода публичное.
- Метод может три возможные сигнатуры: без агрументов, `*http.Request`, `*http.Request, *interface{}`.
- Метод имеет два типа возвращаемого значения interface{}, error.

Все другие методы игнорируются.
