package main

// An example Bubble Tea server. This will put an ssh session into alt screen
// and continually print up to date terminal information.

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"
	"github.com/gammazero/deque"
	"github.com/google/uuid"
	"github.com/muesli/termenv"
)

const (
	host = "0.0.0.0"
	port = "2227"
)

type Message struct {
	username, text, time string
}

type app struct {
	idToClient map[uuid.UUID]*tea.Program
	// messages      string
	messagesDeque *deque.Deque[Message]
	// clearPassword string
	// messages   [20]Message
}

type NewMessageMsg struct {
	// text string
}

func (a *app) ProgramHandler(s ssh.Session) *tea.Program {

	// model := initialModel(a.chains)
	// This should never fail, as we are using the activeterm middleware.

	data := []byte(strings.Split(s.RemoteAddr().String(), ":")[0])
	hash := sha256.Sum256(data)
	// id := fmt.Sprintf("%x", hash[0:5])
	// fmt.Println(id)
	// fmt.Printf("SHA-256 hash: %x\n", hash)

	// password := []byte("clear_all")
	// passwordHash := sha256.Sum256(password)
	// fmt.Println(fmt.Sprintf("%x", passwordHash))

	pty, _, _ := s.Pty()

	renderer := bubbletea.MakeRenderer(s)
	fmt.Println(renderer.ColorProfile().Name())
	messageInput := textinput.New()
	messageInput.Placeholder = "press spacebar to type"
	messageInput.Width = pty.Window.Width

	messageInput.PlaceholderStyle = renderer.NewStyle().Foreground(lipgloss.Color("#3C3C3C"))
	// messageInput.Cursor.TextStyle.Renderer(renderer).Background(lipgloss.Color("0xfffff"))
	// messageInput.Cursor.SetChar("D")
	messageInput.Cursor.Style = renderer.NewStyle().Background(lipgloss.AdaptiveColor{Light: "255", Dark: "0"})

	model := model{
		term:         pty.Term,
		profile:      renderer.ColorProfile().Name(),
		width:        pty.Window.Width,
		height:       pty.Window.Height,
		messageInput: messageInput,
		id:           uuid.New(),
		username:     s.User() + fmt.Sprintf("_%x", hash[0:2]),
		renderer:     renderer,
		app:          a,
	}
	model.viewport = viewport.New(model.width, model.height-1)
	// model.viewport.YPosition = 0
	model.viewport.MouseWheelEnabled = true

	// model.viewport.SetContent(model.stringifyMessages())
	// model.viewport.GotoBottom()

	message := Message{
		username: "",
		time:     "",
		text:     renderer.NewStyle().Foreground(lipgloss.Color("#3C3C3C")).Render(fmt.Sprintf("%s has entered the chat.", model.username)),
	}

	a.messagesDeque.PushBack(message)

	model.viewport.SetContent(model.stringifyMessages())
	model.viewport.GotoBottom()

	p := tea.NewProgram(model, tea.WithInput(s), tea.WithOutput(s), tea.WithAltScreen(), tea.WithMouseCellMotion())
	a.idToClient[model.id] = p
	model.updateClients(NewMessageMsg{})
	return p
}

// Just a generic tea.Model to demo terminal information of ssh.
type model struct {
	term         string
	profile      string
	width        int
	height       int
	messageInput textinput.Model
	id           uuid.UUID
	app          *app
	username     string
	viewport     viewport.Model
	renderer     *lipgloss.Renderer
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// var cmd tea.Cmd
	var cmd1, cmd2 tea.Cmd
	var cmds []tea.Cmd
	m.messageInput, cmd1 = m.messageInput.Update(msg)
	m.viewport, cmd2 = m.viewport.Update(msg)
	cmds = append(cmds, cmd1, cmd2)
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			quit := Message{
				username: "",
				time:     "",
				text:     m.renderer.NewStyle().Foreground(lipgloss.Color("#3C3C3C")).Render(fmt.Sprintf("%s has left the chat.", m.username)),
			}
			delete(m.app.idToClient, m.id)
			// client := m.app.idToClient[m.id]
			m.app.messagesDeque.PushBack(quit)
			m.updateClients(NewMessageMsg{})
			m.viewport.SetContent(m.stringifyMessages())
			m.viewport.GotoBottom()
			return m, tea.Quit
		case " ":
			if !m.messageInput.Focused() {
				focusCmd := m.messageInput.Focus()
				cmds = append(cmds, focusCmd)
				m.messageInput.Placeholder = "press enter to send"
				// m.messageInput.
				// m.messageInput.SetValue("")
			}

		// case "420":
		// 	m.app.messagesDeque.Clear()
		case "enter":
			if m.messageInput.Focused() {
				if m.app.messagesDeque.Len() >= 20 {
					m.app.messagesDeque.PopFront()
				}

				// fmt.Println(m.messageInput.Value())
				var message Message
				if m.messageInput.Value() == "clear_all69" {
					m.app.messagesDeque.Clear()
					message = Message{
						username: "",
						time:     "",
						text:     m.renderer.NewStyle().Foreground(lipgloss.Color("#3C3C3C")).Render("chat cleared"),
					}
				} else {
					message = Message{
						username: m.username,
						time:     "@" + time.Now().Format("15:04:05"),
						text:     "\n  ↳" + m.messageInput.Value(),
					}
				}

				m.messageInput.Reset()
				m.messageInput.Blur()
				m.messageInput.Placeholder = "press spacebar to type"

				m.app.messagesDeque.PushBack(message)

				m.viewport.SetContent(m.stringifyMessages())
				m.viewport.GotoBottom()

				m.updateClients(NewMessageMsg{})

			}

		}

	case NewMessageMsg:

	}
	var messages string
	// fmt.Println(m.app.messagesDeque.Len())
	for i := range m.app.messagesDeque.Len() {
		message := m.app.messagesDeque.At(i)
		messages += "\n" + message.username + message.time + message.text

	}

	// m.messageInput, cmd1 = m.messageInput.Update(msg)

	// fmt.Println(cmd1, cmd2)

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	// s := fmt.Sprintf("Your term is %s\nYour window size is %dx%d\nBackground: %s\nColor Profile: %s", m.term, m.width, m.height, m.bg, m.profile)
	// return m.txtStyle.Render(s) + "\n\n" + m.quitStyle.Render("Press 'q' to quit\n")
	// var messages string
	// // fmt.Println(m.app.messagesDeque.Len())
	// for i := range m.app.messagesDeque.Len() {
	// 	message := m.app.messagesDeque.At(i)
	// 	messages += "\n" + message.username + message.time + message.text + "\n"

	// }
	// m.viewport.SetContent(messages)
	// fmt.Println(messages)
	// if m.app.messagesDeque.Len() > 0 {

	// 	messages = m.app.messagesDeque.Front()

	// }
	// input := lipgloss.PlaceVertical(m.height-m.app.messagesDeque.Len()*2, lipgloss.Bottom, m.messageInput.View())
	// m.viewport.GotoBottom()
	// m.viewport.SetContent(m.stringifyMessages())
	// m.viewport.GotoBottom()

	return lipgloss.JoinVertical(lipgloss.Bottom, m.viewport.View(), m.messageInput.View())

	// return m.viewport.View() + m.messageInput.View()
}

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	a := new(app)
	a.idToClient = make(map[uuid.UUID]*tea.Program)
	a.messagesDeque = new(deque.Deque[Message])
	// a.clearPassword = "90239a30e767caddf585ece44f0d572d659631b9fce0de13c713f101f6103863"
	s, err := wish.NewServer(
		wish.WithAddress(net.JoinHostPort(host, port)),
		// wish.WithHostKeyPath(fmt.Sprint(home, "/.ssh/chat-app")),
		wish.WithHostKeyPath(fmt.Sprint(home, "/.ssh/chat-app")),
		// wish.WithPublicKeyAuth(publicKeyAuthHandler),
		wish.WithMiddleware(
			bubbletea.MiddlewareWithProgramHandler(a.ProgramHandler, termenv.ANSI256),
			activeterm.Middleware(), // Bubble Tea apps usually require a PTY.
			logging.Middleware(),
		),
	)
	if err != nil {
		log.Error("Could not start server", "error", err)
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	log.Info("Starting SSH server", "host", host, "port", port)
	go func() {
		if err = s.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			log.Error("Could not start server", "error", err)
			done <- nil
		}
	}()

	<-done
	log.Info("Stopping SSH server")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer func() { cancel() }()
	if err := s.Shutdown(ctx); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
		log.Error("Could not stop server", "error", err)
	}
}

func (m *model) updateClients(msg NewMessageMsg) {
	for id, client := range m.app.idToClient {
		if id != m.id {
			client.Send(NewMessageMsg{})
		}

	}
}

func publicKeyAuthHandler(ctx ssh.Context, key ssh.PublicKey) bool {
	fmt.Println(key)
	return true
}

func (m *model) stringifyMessages() string {
	var messages string
	// fmt.Println(m.app.messagesDeque.Len())
	for i := range m.app.messagesDeque.Len() {
		message := m.app.messagesDeque.At(i)
		messages += "\n" + message.username + message.time + message.text + "\n"

	}
	return messages
}
