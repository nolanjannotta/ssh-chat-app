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
	idToClient    map[uuid.UUID]*tea.Program
	messagesDeque *deque.Deque[Message]
}

type NewMessageMsg struct {
	// text string
}

type model struct {
	width, height int
	messageInput  textinput.Model
	id            uuid.UUID
	app           *app
	username      string
	viewport      viewport.Model
	renderer      *lipgloss.Renderer
	color         string
}

func (a *app) ProgramHandler(s ssh.Session) *tea.Program {
	model := model{}

	ipBytes := []byte(strings.Split(s.RemoteAddr().String(), ":")[0])
	ipHash := sha256.Sum256(ipBytes)

	colorBytes := []byte(s.User() + strings.Split(s.RemoteAddr().String(), ":")[0])
	colorHash := sha256.Sum256(colorBytes)

	pty, _, _ := s.Pty()

	renderer := bubbletea.MakeRenderer(s)
	messageInput := textinput.New()
	viewport := viewport.New(pty.Window.Width, pty.Window.Height-1)

	// textinput.NewModel
	messageInput.Placeholder = "press spacebar to type"
	messageInput.Width = pty.Window.Width

	messageInput.PlaceholderStyle = renderer.NewStyle().Foreground(lipgloss.Color("#3C3C3C"))
	messageInput.Cursor.Style = renderer.NewStyle().Background(lipgloss.AdaptiveColor{Light: "255", Dark: "0"})

	model.width = pty.Window.Width
	model.height = pty.Window.Height
	model.messageInput = messageInput
	model.id = uuid.New()
	model.username = s.User() + fmt.Sprintf("_%x", ipHash[0:2])
	model.renderer = renderer
	model.app = a
	model.viewport = viewport
	model.color = fmt.Sprintf("%x", colorHash[0:3])

	fmt.Println(model.color)
	message := Message{}
	message.text = renderer.NewStyle().Foreground(lipgloss.Color("#3C3C3C")).Render(fmt.Sprintf("%s has entered the chat.", model.username))

	a.messagesDeque.PushBack(message)

	model.viewport.SetContent(model.stringifyMessages())
	model.viewport.GotoBottom()

	p := tea.NewProgram(model, tea.WithInput(s), tea.WithOutput(s), tea.WithAltScreen(), tea.WithMouseCellMotion())
	a.idToClient[model.id] = p
	model.updateClients(NewMessageMsg{})
	return p
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

			}

		case "enter":
			if m.messageInput.Focused() {
				if m.app.messagesDeque.Len() >= 20 {
					m.app.messagesDeque.PopFront()
				}
				m.renderer.ColorProfile()
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
						username: m.renderer.NewStyle().Foreground(lipgloss.Color("#" + m.color)).Render(m.username),
						time:     "@" + time.Now().UTC().Format("15:04:05") + " UTC",
						text:     "\n  ↳" + m.messageInput.Value(),
					}
					fmt.Println("#" + m.color)
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
		m.viewport.SetContent(m.stringifyMessages())
		m.viewport.GotoBottom()

	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	return m.viewport.View() + "\n" + m.messageInput.View()
}

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	a := new(app)
	a.idToClient = make(map[uuid.UUID]*tea.Program)
	a.messagesDeque = new(deque.Deque[Message])
	s, err := wish.NewServer(
		wish.WithAddress(net.JoinHostPort(host, port)),
		wish.WithHostKeyPath(fmt.Sprint(home, "/.ssh/chat-app")),
		wish.WithMiddleware(
			bubbletea.MiddlewareWithProgramHandler(a.ProgramHandler, termenv.TrueColor),
			activeterm.Middleware(),
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

func (m *model) stringifyMessages() string {
	var messages string
	for i := range m.app.messagesDeque.Len() {
		message := m.app.messagesDeque.At(i)
		messages += "\n" + message.username + message.time + message.text + "\n"

	}
	return messages
}
