package main

import (
	"os"
	"log"
	"fmt"
	"context"
	"time"
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

func submitWatcher(submissions chan string) {
	for {
		select {
			case s := <-submissions:
				loggo.Info("caught submission, use Collect() on", 
					"formName", s)
				switch s {
				case "SampleForm":
					loggo.Info("sampleForm", "contents", sampleForm.Collect())
				case "CustomForm":
					loggo.Info("customForm", "contents", customForm.Collect())
				}
		}
	}
}

// so we can easily reference them in submission goroutine
var sampleForm *ugform.Form
var customForm *ugform.Form


func main() {
	//setLogger(true, "main.ugform.log.json", "info")
	setLogger(true, "main.ugform.log.json", "debug")
	// log15 logger can be passed to ugform package 
	// otherwise all package logs are discarded
	ugform.Loggo = loggo
	var err error
	// first we need a screen to pass to the form
	s, err := tcell.NewScreen()
	// make a quit channel for the main polling function
	quit := make(chan struct{})
	submit := make(chan string)
	if err != nil {
		log.Fatalf("%+v", err)
	}
	if err := s.Init(); err != nil {
		log.Fatalf("%+v", err)
	}
	s.SetStyle(tcell.StyleDefault)

	// create a new form 
	sampleForm = ugform.NewForm(s)
	sampleForm.Name = "SampleForm"
	// add content to the form, there's a handy func for this for demo
	err = ugform.AddSampleTextBoxes(sampleForm)
	if err != nil {
		log.Fatalf("%+v", err)
	}
	// now show how to create custom forms
	customForm = ugform.NewForm(s)
	customForm.Name = "CustomForm"
	err = customForm.AddTextBox(
		&ugform.AddTextBoxInput{
			Name: "password",
			Description: "Password: ",
			PositionX: 45,
			PositionY: 20,
			Width: 100,
			Height: 2,
			StyleCursor: ugform.StyleCursor("white"),
			StyleFill: ugform.StyleFill("green"),
			StyleText: ugform.StyleHelper("red", "green"),
			StyleDescription: ugform.StyleHelper("orange", "gray"),
			ShowDescription: true,
			Password: true,
		},
	)
	if err != nil {
		log.Fatalf("%+v", err)
	}
	// you can shift position of the box after creation
	// by calling the ShiftXY method
	sampleForm.ShiftXY(3,20)
	// calling start will draw the boxes
	loggo.Info("starting forms")
	sampleForm.Start()
	customForm.Start()
	// set up optional context so you can regain control 
	// of the form's blocking PollEvent
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// start a submission watcher so submissions don't block
	go submitWatcher(submit)
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
							go sampleForm.Poll(ctx, formInterrupt, submit)
							loggo.Info("pausing main poll")

							// if you want you can wrestle
							// control back from form Poll
							// by cancelling context
							time.Sleep(5*time.Second)
							cancel()
							// otherwise wait for form
							// to close the interrupt
							<-formInterrupt
							loggo.Info("resuming main poll")
							// you'll have to reset the context
							// if you want to keep using it
							ctx, cancel = context.WithCancel(
								context.Background())
						// a different keystroke can activate
						// a different form
						case csr("k"):
							formInterrupt := make(chan struct{})
							go customForm.Poll(ctx, formInterrupt, submit)
							<-formInterrupt
						// have a keystroke that moves the form
						// around
						case csr("u"):
							sampleForm.ClearShiftXY(0,-3)
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
	// collect results from the forms as a map[string]string
	// where key is the field name and value is the textbox
	// contents
	fmt.Println("Results from sampleForm:")
	for k, v := range sampleForm.Collect() {
		fmt.Printf("	%s: '%s'\n", k, v)
	}
	fmt.Println("Results from customForm:")
	for k, v := range customForm.Collect() {
		fmt.Printf("	%s: '%s'\n", k, v)
	}
	os.Exit(0)
}
