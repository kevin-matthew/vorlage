package main

import "fmt"

type lmerror interface {
	error
	ErrorId() int
	ErrorDescription() string
	ErrorSubject() string
	ErrorCause() error
	ErrorFix() string
}

/* A struct that has the exclusive purpose of reporting faulty input is
 * known as an *errT*.
 * Describing what happens when errors occour is half of your job. The
 * other half is describing what happens when errors do not occour. Thus,
 * one can assume that the role of a software developer is simply a
 * digital error expert.
 */
type errT struct {

	// all possible errors in a given application must have their own unique
	// int id. You should make a file in your namespace called an 'errors' file
	// that contains a list of enum ids.
	// why the input was faulty or had caused fault, must be human readable
	Id          int
	Description string

	// the error that caused this error, required in Accumulative Error Reporting
	Cause error

	// The input that was directly the cause of the error.
	// The caller must recongize the subject by its content
	SubjectFmt  string
	SubjectArgs []interface{}

	// A solution is a human-readable string that should be used when reporting
	// the error to a human.
	Fix string
}

func (e errT) ErrorId() int {
	return e.Id
}

func (e errT) ErrorDescription() string {
	return e.Description
}

func (e errT) ErrorSubject() string {
	return fmt.Sprintf(e.SubjectFmt, e.SubjectArgs...)
}

func (e errT) ErrorCause() error {
	return e.Cause
}

func (e errT) ErrorFix() string {
	return e.Fix
}

var _ lmerror = errT{}

func FormatLmErr(e errT) string {
	var f string
	var args []interface{}

	// the subject
	s := e.ErrorSubject()
	if s != "" {
		f += "\"%s\" - "
		args = append(args, s)
	}

	// source
	f += "%s"
	args = append(args, e.ErrorDescription())

	// the id
	//f += "[errT#%d]"
	//args = append(args, e.ErrorId())

	// the solution
	fx := e.ErrorFix()
	if fx != "" {
		f += " (%s)"
		args = append(args, fx)
	}

	// the blame
	c := e.ErrorCause()
	if c != nil {
		f += ": %s"
		args = append(args, c.Error())
	}

	return fmt.Sprintf(f, args...)
}

func (e errT) Error() string {
	return FormatLmErr(e)
}

// New and Newf generates new (original) errors that conforms to the standards
// set in software-craft in the Ellem opman.
// When calling this funciton, although this may sound odd, you want to just
// make up a random and unique id, for example:
//   errors.New(9518753, "something bad happened" ... )
// Just make sure this id is over 66000 and is unique. This is to allow the
// programmer who is calling your code to handle your errors based on a unique
// id rather than relying on the description (which can sometimes change).
//
// Note that sometimes cause, fix, and subjectf won't always be nessacary.
//
// Newf does the exact same thing as New execpt allows you to use formatting
// when specifiying a subject, for example:
//   errors.Newf(3250682, "invalid username", nil, "check the username and try again", "the username was %s", username)
func lmerrorNew(id int, description string, cause error, fix string, subject string) lmerror {
	return errT{id, description, cause, "%s", []interface{}{subject}, fix}
}
func lmerrorNewf(id int, description string, cause error, fix string, subjectf string, subjectArgs ...interface{}) lmerror {
	return errT{id, description, cause, subjectf, subjectArgs, fix}
}
