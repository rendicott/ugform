package main

import (
	"os"
	"log"
	"github.com/inconshreveable/log15"
	"github.com/gdamore/tcell/v2"
	"github.com/rendicott/ugform"
)

// loggo is the global logger
var loggo log15.Logger

func csr(s string) int32 {
	if len(s) == 0 {
		s = " "
	}
	runes := []rune(s)
	return runes[0]
}

// setLogger sets up logging globally for the packages involved
// in the gossamer runtime.
func setLogger(daemonFlag bool, logFileS, loglevel string) {
	loggo = log15.New()
	if daemonFlag && loglevel == "debug" {
		loggo.SetHandler(
			log15.LvlFilterHandler(
				log15.LvlDebug,
				log15.Must.FileHandler(logFileS, log15.JsonFormat())))
	} else if daemonFlag && loglevel == "info" {
		loggo.SetHandler(
			log15.LvlFilterHandler(
				log15.LvlInfo,
				log15.Must.FileHandler(logFileS, log15.JsonFormat())))
	} else if loglevel == "debug" && !daemonFlag {
		// log to stdout and file
		loggo.SetHandler(log15.MultiHandler(
			log15.StreamHandler(os.Stdout, log15.LogfmtFormat()),
			log15.LvlFilterHandler(
				log15.LvlDebug,
				log15.Must.FileHandler(logFileS, log15.JsonFormat()))))
	} else {
		// log to stdout and file
		loggo.SetHandler(log15.MultiHandler(
			log15.LvlFilterHandler(
				log15.LvlInfo,
				log15.StreamHandler(os.Stdout, log15.LogfmtFormat())),
			log15.LvlFilterHandler(
				log15.LvlInfo,
				log15.Must.FileHandler(logFileS, log15.JsonFormat()))))
	}
}



func main() {
	setLogger(true, "main.ugform.log.json", "info")
	// log15 logger can be passed to ugform package 
	// otherwise all package logs are discarded
	ugform.Loggo = loggo
	var err error
	// first we need a screen to pass to the form
	s, err := tcell.NewScreen()
	// make a quit channel for the main polling function
	quit := make(chan struct{})
	if err != nil {
		log.Fatalf("%+v", err)
	}
	if err := s.Init(); err != nil {
		log.Fatalf("%+v", err)
	}
	s.SetStyle(tcell.StyleDefault)

	// create a new form 
	sampleForm := ugform.NewForm(s)
	// add content to the form, there's a handy func for this for demo
	err = ugform.AddSampleTextBoxes(sampleForm)
	if err != nil {
		log.Fatalf("%+v", err)
	}
	// now show how to create custom forms
	customForm := ugform.NewForm(s)
	err = customForm.AddTextBox(
		&ugform.AddTextBoxInput{
			Name: "spaces are supported",
			Description: "All inputs are returned as strings",
			PositionX: 45,
			PositionY: 20,
			Width: 100,
			Height: 2,
			StyleCursor: ugform.StyleCursor("white"),
			StyleFill: ugform.StyleFill("green"),
			StyleText: ugform.StyleText("red"),
			StyleDescription: ugform.StyleHelper("orange", "gray"),
			ShowDescription: true,
		},
	)
	if err != nil {
		log.Fatalf("%+v", err)
	}
	loggo.Info("starting forms")
	// calling start will draw the boxes
	sampleForm.Start()
	customForm.Start()
	// start a main even poller 
	go func() {
		for {
			ev := s.PollEvent()
			switch ev := ev.(type) {
			case *tcell.EventKey:
				switch ev.Key() {
				case tcell.KeyRune:
					switch ev.Rune() {
						// detect when user presses a desired key
						// in this case it's 'j'
						case csr("j"):
							// now pass polling rights to 
							// the form with the interrupt
							// channel you want to wait for
							formInterrupt := make(chan struct{})
							go sampleForm.Poll(formInterrupt)
							loggo.Info("pasuing main poll")
							<-formInterrupt
							loggo.Info("resuming main poll")
						case csr("k"):
							formInterrupt := make(chan struct{})
							go customForm.Poll(formInterrupt)
							<-formInterrupt
					}
				// define a main poller exit routine
				// but keep in mind this won't work
				// while form is polling
				case tcell.KeyCtrlC:
					close(quit)
					return
			default:
				loggo.Info("detected stroke", "keyStroke", ev.Name())
				}
			}
		}
	}()
	// have the main wait here for main polling loop to close
	<-quit
	// clean up and close out screen
	s.Fini()
	os.Exit(0)
}
