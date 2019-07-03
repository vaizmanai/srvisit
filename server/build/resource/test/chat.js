let socket;

function init() {

    let wsURI = "ws://" + window.location.host + "/v2/api/chat";

    const socketMessageListener = (e) => {
        let wsMessage = JSON.parse(e.data);
        console.log(wsMessage);
        if (wsMessage.Type === 1) {
            let chatMessage = JSON.parse(wsMessage.Data);
            content.innerHTML += `<div>${chatMessage.Pid}: ${decodeURI(chatMessage.Text)}</div>`;
        }
    };

    const socketOpenListener = (e) => {
        console.log("connected to " + wsURI);
    };

    const socketCloseListener = (e) => {
        if (socket) {
            console.log("connection closed: " + e);
        }
        socket = new WebSocket(wsURI);
        socket.addEventListener('open', socketOpenListener);
        socket.addEventListener('message', socketMessageListener);
        socket.addEventListener('close', socketCloseListener);
    };

    socketCloseListener();

    let wsMessagePing = {Type: 0, Data: "ping"};
    setInterval(function () {
            if (socket !== undefined) socket.send(JSON.stringify(wsMessagePing));
        }, 3000
    )
}

function sendMessage() {
    if (socket !== undefined) {
        let chatMessage = {Pid: pid.value, Text: encodeURI(text.value)};
        let wsMessage = {Type: 1, Data: JSON.stringify(chatMessage)};
        socket.send(JSON.stringify(wsMessage));
    }
}
