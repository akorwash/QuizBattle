package websockets

type messageContainer struct {
	Message  string
	Fullname string
}

type Message struct {
	id   uint64
	data []byte
}

// Hub maintains the set of active clients and broadcasts messages to the
// clients.
type Hub struct {
	active bool
	// Registered clients.
	clients map[*Client]bool

	// Inbound messages from the clients.
	broadcast chan Message

	// Inbound messages from the clients.
	broadcastJSONmessage chan messageContainer

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client
}

//NewHub to do
func NewHub() *Hub {
	return &Hub{
		active:     true,
		broadcast:  make(chan Message),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}
}

//Close to do
func (h *Hub) Close() {
	h.active = false
}

//Run to do
func (h *Hub) Run() {
	for {
		if !h.active {
			for client := range h.clients {
				close(client.send)
				delete(h.clients, client)
			}
			return
		}
		select {
		case client := <-h.register:
			h.clients[client] = true
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		case message := <-h.broadcast:
			for client := range h.clients {
				if message.id == 0 {
					select {
					case client.send <- message.data:
					default:
						close(client.send)
						delete(h.clients, client)
					}
				} else {
					if client.UserID != message.id {
						select {
						case client.send <- message.data:
						default:
							close(client.send)
							delete(h.clients, client)
						}
					}
				}

			}
		}
	}
}
