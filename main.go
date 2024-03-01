package main

import (
	"fmt"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
	"github.com/tarm/serial"
	"io"
	"log"
	"os"
	"strconv"
	"time"
)

type appState int

const (
	INPUT appState = iota
	NONE

	maxWidth = 40
)

var (
	stopch = make(chan struct{})
	stdout = make(chan string, 10)
	stdin  = make(chan string, 10)

	// styles
	errorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FA2333"))
	stdoutStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#81FCBF"))
	accentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#e88c5f"))
	docStyle    = lipgloss.NewStyle().Margin(1, 2)
)

type PortInfo struct {
	name      string
	baut_rate int
}

type model struct {
	state         appState
	port          serial.Port
	port_info     PortInfo
	textInput     textinput.Model
	serial_output []string
	display_text  string
	width         int
	height        int
}

func initialModel(port_name string, baud_rate int) model {
	// serial setup
	var config = &serial.Config{
		Name:        port_name,
		Baud:        baud_rate,
		ReadTimeout: time.Duration(1000),
	}

	port, err := serial.OpenPort(config)
	if err != nil {
		log.Fatal(err)
	}

	// text imput
	ti := textinput.New()
	ti.Placeholder = "type"
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 20
	return model{
		state:         INPUT,
		textInput:     ti,
		port:          *port,
		serial_output: make([]string, 0),
		port_info: PortInfo{
			name:      port_name,
			baut_rate: baud_rate,
		},
	}
}

func (m model) handle_serial(stopch <-chan struct{}) {
	for {
		select {
		case <-stopch:
			return
		default:
			// sending data
			{
				select {
				case data := <-stdin:
					_, err := m.port.Write([]byte(data + "\n"))
					if err != nil {
						stdout <- errorStyle.Render(err.Error())
					}
				default:
				}
			}

			// reciveing data
			{
				buff := make([]byte, 1024)
				n, err := m.port.Read(buff)
				if err != nil {
					if err != io.EOF {
						stdout <- errorStyle.Render(err.Error())
					}
					continue
				}
				var msg = string(buff[:n])
				if msg != "EOF" {
					stdout <- string(buff[:n])
				}
			}
		}
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd
	switch msg := msg.(type) {

	case tea.KeyMsg:

		switch msg.String() {

		case "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.state += 1
			if m.state > NONE {
				m.state = 0
			}
		case "enter":
			var v = m.textInput.Value()
			if v == "exit" || v == "quit" {
				return m, tea.Quit
			}
			if v != "" {
				stdin <- v
				m.textInput.SetValue("")
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		cmds = append(cmds, tea.ClearScreen)
	}

	// updates data
	select {
	case data := <-stdout:
		m.serial_output = append(m.serial_output, data)
		m.display_text += data
		cmds = append(cmds, tea.ClearScreen)
	default:

	}
	if m.state == INPUT {
		m.textInput, cmd = m.textInput.Update(msg)
		cmds = append(cmds, cmd)

	}
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func port_info(m *model) string {
	return fmt.Sprintf("port: %s, baut rate: %s",
		accentStyle.Bold(true).Render(m.port_info.name),
		accentStyle.Bold(true).Render(strconv.Itoa(m.port_info.baut_rate)))
}

func (m model) View() string {
	return wordwrap.String(fmt.Sprintf("%s\n%s\n%s",
		stdoutStyle.Render(m.display_text),
		port_info(&m),
		m.textInput.View()), min(m.width, maxWidth))
}

func main() {
	if len(os.Args) != 3 {
		fmt.Println(errorStyle.Bold(true).Render(fmt.Sprintf("Usage: %s <port> <baud_rate>", os.Args[0])))
		os.Exit(1)
	}
	baut_rate, err := strconv.Atoi(os.Args[2])
	if err != nil {
		log.Fatal(err)
	}

	var m = initialModel(os.Args[1], baut_rate)
	go m.handle_serial(stopch)
	p := tea.NewProgram(m)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
	}
	err = m.port.Close()
	if err != nil {
		log.Print(err)
	}

	close(stopch)
}
