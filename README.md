Chat App Demo with web sockets
=========================
The `Chat` app demonstrates.

# ENV VARIABLES

Setup the following env variabels for the app to run.
`FB_CLIENT_ID` -> Facebook App Client ID

`FB_CLIENT_SECRET` -> Facebook App Client secret

# Install dependency

Use dep as dependency manager.

* Using channels to implement a chat room with Websockets.

Here's a quick summary of the structure:

``` bash
	chat/app/
		chatroom	       # Chat room routines
			chatroom.go

		controllers
			app.go         # The login screen, allowing user to Sign-in
			websocket.go   # Handlers for the "Websocket" chat demo

		views
			                # HTML and Javascript

```
# Chat Room Background

First, let's look at how the chat room is implemented.

The chat room runs as an independent `go-routine`, started on initialization:

```go
func init() {
	go chatroom()
}
```

The `chatroom()` function simply selects on three channels to execute the requested action.

```go
var (
	// Send a channel here to get room events back.  It will send the entire
	// archive initially, and then new messages as they come in.
	subscribe = make(chan (chan<- Subscription), 10)
	// Send a channel here to unsubscribe.
	unsubscribe = make(chan (<-chan Event), 10)
	// Send events here to publish them.
	publish = make(chan Event, 10)
)

func chatroom() {
	archive := list.New()
	subscribers := list.New()

	for {
		select {
		case ch := <-subscribe:
			// Add subscriber to list and send back subscriber channel + chat log.
		case event := <-publish:
			// Send event to all subscribers and add to chat log.
		case unsub := <-unsubscribe:
			// Remove subscriber from subscriber list.
		}
	}
}
```

Let's examine how each of those channel functions are implemented.

### Subscribe 

```go
case ch := <-subscribe:
    var events []Event
    for e := archive.Front(); e != nil; e = e.Next() {
        events = append(events, e.Value.(Event))
    }
    subscriber := make(chan Event, 10)
    subscribers.PushBack(subscriber)
    ch <- Subscription{events, subscriber}
```

A `Subscription` is created with two properties:

* The chat log (archive)
* A channel that the subscriber can listen on to get new messages.

The `Subscription` is then sent to the channel that subscriber supplied.


### Publish

```go
case event := <-publish:
    for ch := subscribers.Front(); ch != nil; ch = ch.Next() {
        ch.Value.(chan Event) <- event
    }
    if archive.Len() >= archiveSize {
        archive.Remove(archive.Front())
    }
    archive.PushBack(event)
```

The `Published event` is sent to the subscribers' channels one by one.  
- The `event` is added to the `archive`, which is trimmed if necessary.

### Unsubscribe

```go
case unsub := <-unsubscribe:
    for ch := subscribers.Front(); ch != nil; ch = ch.Next() {
        if ch.Value.(chan Event) == unsub {
            subscribers.Remove(ch)
        }
    }
```

The `Subscriber` channel is removed from the list.

## Handlers

Now that the `Chat Room` channels exist, lets examine how the handlers
expose that functionality using `WebSockets`.

### Websocket

The Websocket chat room
opens a websocket connection as soon as the
user has loaded the page.

```js
// Create a socket
var socket = new WebSocket('ws://127.0.0.1:9000/websocket/room/socket?user={{.user}}');

// Message received on the socket
socket.onmessage = function(event) {
    display(JSON.parse(event.data));
}

$('#send').click(function(e) {
    var message = $('#message').val();
    $('#message').val('');
    socket.send(message);
});
```

The first thing to do is to subscribe to new events, join the room, and send
down the archive.  Here is what `websocket.go` looks like:

```go
func (c WebSocket) RoomSocket(user string, ws *websocket.Conn) revel.Result {
	// Join the room.
	subscription := chatroom.Subscribe()
	defer subscription.Cancel()

	chatroom.Join(user)
	defer chatroom.Leave(user)

	// Send down the archive.
	for _, event := range subscription.Archive {
		if websocket.JSON.Send(ws, &event) != nil {
			// They disconnected
			return nil
		}
	}
	....
```


Next, we have to listen for new events from the subscription.  However, the
websocket library only provides a blocking call to get a new frame.  To select
between them, we have to wrap it.

```go
// In order to select between websocket messages and subscription events, we
// need to stuff websocket events into a channel.
newMessages := make(chan string)
go func() {
    var msg string
    for {
        err := websocket.Message.Receive(ws, &msg)
        if err != nil {
            close(newMessages)
            return
        }
        newMessages <- msg
    }
}()
```

Now we can select for new websocket messages on the `newMessages` channel.

The last bit does exactly that -- it waits for a new message from the websocket
(if the user has said something) or from the subscription (someone else in the
chat room has said something) and propagates the message to the other.

```go
// Now listen for new events from either the websocket or the chatroom.
for {
    select {
    case event := <-subscription.New:
        if websocket.JSON.Send(ws, &event) != nil {
            // They disconnected.
            return nil
        }
    case msg, ok := <-newMessages:
        // If the channel is closed, they disconnected.
        if !ok {
            return nil
        }

        // Otherwise, say something.
        chatroom.Say(user, msg)
    }
}
return nil

```

If we detect the websocket channel has closed, then we just return nil.
