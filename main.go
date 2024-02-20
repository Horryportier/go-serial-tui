package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tarm/serial"
)

var (
	stopch = make(chan struct{})
	stdout = make(chan string, 10)
	stdin  = make(chan string, 10)

	exit = false

	test []string

	// styles
	errorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FA2333"))
	stdoutStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#81FCBF"))
	accentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#e88c5f"))
)

type PortInfo struct {
	name      string
	baut_rate int
}

type model struct {
	port          serial.Port
	port_info     PortInfo
	textInput     textinput.Model
	serial_output string
}

func initialModel(port_name string, baud_rate int) model {
	var config = &serial.Config{
		Name:        port_name,
		Baud:        baud_rate,
		ReadTimeout: time.Duration(1000),
	}

	port, err := serial.OpenPort(config)
	if err != nil {
		log.Fatal(err)
		exit = true
	}

	ti := textinput.New()
	ti.Placeholder = "type"
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 20

	return model{
		textInput:     ti,
		port:          *port,
		serial_output: "",
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
	switch msg := msg.(type) {

	case tea.KeyMsg:

		switch msg.String() {

		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			var v = m.textInput.Value()
			if v != "" {
				stdin <- v
				m.textInput.SetValue("")
			}
		}
	}

	if exit {
		return m, tea.Quit
	}
	select {
	case data := <-stdout:
		test = append(test, data)
		m.serial_output = fmt.Sprintf("%s%s", m.serial_output, data)
	default:
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)

	return m, cmd
}

func port_info(m *model) string {
	return fmt.Sprintf("port: %s, baut rate: %s", 
		accentStyle.Bold(true).Render(m.port_info.name),
		accentStyle.Bold(true).Render(strconv.Itoa(m.port_info.baut_rate)))
}

func (m model) View() string {
	return fmt.Sprintf("%s\n%s\n%s", stdoutStyle.Render(m.serial_output), port_info(&m),
		m.textInput.View(),
	)
}

func main() {
	if len(os.Args) != 3 {
		fmt.Println(errorStyle.Bold(true).Render( fmt.Sprintf("Usage: %s <port> <baud_rate>", os.Args[0])))
		os.Exit(1)
	}
	baut_rate, err := strconv.Atoi(os.Args[2])
	if err != nil {
		log.Fatal(err)
	}

	var m = initialModel(os.Args[1], baut_rate)
	go m.handle_serial(stopch)
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
	}
	err = m.port.Close()
	if err != nil {
		log.Print(err)
	}

	close(stopch)
}
