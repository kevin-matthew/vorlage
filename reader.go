package vorlage

import (
	"io"
	"strings"
)

// helper function for nonConvertedFile.Read
// returns io.EOF if the definition has been completely read from.
func (c *nonConvertedFile) readDefinition(dest []byte) (n int, err error) {
	// are we currently exposing a defnition?
	if c.currentlyReadingDef != nil {
		// we are... so lets read it.
		n, err = c.currentlyReadingDef.Read(dest)
		if err != nil {
			if err != io.EOF {
				return n, err
			}
			// we're done reading the current definition.
			c.currentlyReadingDef = nil
			// pop this defintion from the stack
			newstack := *c.definitionStack
			*c.definitionStack = newstack[:len(newstack)-1]
			return n, io.EOF
		} else {
			// we're not done reading the definition yet.
			return n, nil
		}
	}
	return 0, io.EOF
}

// reads from the source file, or, reads from tmpBuff if that is not empty.
func (c *nonConvertedFile) readSource(dest []byte) (n int, err error) {
	// tmpBuff has priority over the file because tmpBuff is filled with
	// bytes that we previously read from the source file that we cannot read
	// again.
	if len(c.tmpBuff) != 0 {
		// tmpBuff was not empty. we must read from it.
		n = copy(dest, c.tmpBuff)
		c.tmpBuff = c.tmpBuff[n:]
		return n, nil
	}

	// tmpBuff is empty, read from the source file
	n, err = c.sourceFile.Read(dest)
	// increment the total amount of bytes read from the source file
	c.bytesRead += int64(n)
	return n, err
}

func (c *nonConvertedFile) Read(dest []byte) (totalBytes int, err error) {
	var n int

	// first, read any definition that we may be in currently.
	n, err = c.readDefinition(dest)
	totalBytes += n
	if err != io.EOF {
		// err may be nil here, which means that the definition is not
		// done reading.
		// likewise it may be non-nil with an actual error. in which case
		// we need to return it anyways.
		return totalBytes, err
	}
	// truncate the part of the buffer to which we've already written too...
	// you'll see this a lot in this funciton.
	dest = dest[n:]

	// sense we're not reading from a definition, we can read from the source
	// file (or the tmpBuff if available.)
	n, err = c.readSource(dest)
	// we WILL NOT append this bytes to totalBytes. Because if we do there's a chance
	// we've read in a variable. We don't want that to show up for the caller.
	// totalBytes += n (see above why this was commented out)
	if err != nil && err != io.EOF {
		return totalBytes, err
	}

	// now lets check to see if the source file gave us any variables to chew
	// on. Also, You will not understand the following code until you understand
	// drawParseVar.
	nonVarByteCount, pos, cerr := drawParseVar(c.variableReadBuffer,
		dest[:n],
		c.bytesRead)
	// now here we can acutally add these bytes to what has been read.
	totalBytes += nonVarByteCount
	if cerr != nil {
		// some error happened that made parsing impossible, such as a missing
		// suffix.
		return totalBytes, *cerr
	}
	if nonVarByteCount == n {
		// the entire buffer was found to have no variables in it.
		// (err is returned because it may be an io.EOF from previous calls)
		return totalBytes, err
	}
	if pos != nil {
		// if we're in here... we have scanned in a full variable that follows
		// all the syntax rules.

		// Before we begin reading the variable's definition, we must ask:
		// did the buffer (dest) pick up anything AFTER the variable?
		//
		// (ie dest[:n] = "123$(varible)abc")
		//                    ^         ^  ^
		//                   (a)       (b)(c)
		//
		//  (a) = position of nonVarByteCount
		//  (b) = position of nonVarByteCount + len(pos.fullName)
		//  (c) = position of n (length of string)
		//
		// we need to save the extra (ie "abc") to tmpBuff because
		// dest will be used to read-in the variable and will in turn all
		// content that was read after the variable from the file.
		if n > nonVarByteCount+len(pos.fullName) {

			// so in here we now that n (c) is bigger than (b). Which means
			// there's something after the variable that we've scanned in.
			// lets move it all into tmpBuff.

			// calculate the remaining buffer length (c - b)
			remainingBuffLen := n - (nonVarByteCount + len(pos.fullName))
			var newtmpbuf []byte

			// bug fix for the "forgotten tmp buffer" problem:
			//
			// there's a possibility that tmpbuff
			// currently has stuff in it that hasn't been read into dest.
			// this happens when the previous/parent
			// call had put a valid variable placed in tmpbuff, (to which this
			// current call is now processing) in order to process a preceeding
			// variable.
			// for instance
			//
			//    let $(var1) = vvvv1111,
			//    let $(var2) = vvvv2222,
			//    and let destlen = 24
			//    call1 - source (scanned in from file): "$(var1)...$(var2)...abcd" (24 chars)
			//    call1 - stored into tmpbuf: "...$(var2)...abcd" (17 chars)
			//    call1 - stored into dest: "vvvv1111" (8 chars)
			//
			//   at this point, call1 has written 8 character to dest. this means
			//   call2 must deal with a len(dest) of 16... BUT it is handed a tmpbuf
			//   with a len of 17. so lets continue to see what this causes:
			//
			//    call2 - source (scanned in from tmpbuff): "...$(var2)...abc" (16 characters... notice the absense of 'd'
			//                                                                  because dest didn't have enough room to
			//                                                                  pull in the entirety of tmpbuf.)
			//
			//   now if do not apply this fix, what happens is vorlage detects $(var2)
			//   as well as content after ("...abc"), to which will replace the
			//   tmpbuf and completely forget about the previous non-emptied
			//   tmpbuf:
			//
			//
			//    call2 - stored into tmpbuf: "...abc" (6 chars, if we hadn't included the following if statment,
			//                                          that 'd' character now ceases to exist as we've just replaced
			//                                          the last tmpbuf which still had a 'd' in it.)
			if len(c.tmpBuff) != 0 {
				remainingBuffLen += len(c.tmpBuff)
			}

			// make the tmp buff the size of everything after the variable.
			newtmpbuf = make([]byte, remainingBuffLen)
			// copy everything after that variable into that buffer.
			copyNew := copy(newtmpbuf, dest[nonVarByteCount+len(pos.fullName):n])

			// see "forgotten tmp buffer" problem above
			copy(newtmpbuf[copyNew:], c.tmpBuff)

			c.tmpBuff = newtmpbuf
		}

		// first go back to the Document and find this variable's definition
		var definitionError *Error
		def, derr := c.sourceDocument.define(*pos)
		if derr != nil {
			var ok bool
			if definitionError, ok = derr.(*Error); ok {
				// we need to handle these errors, as the solution to each
				// of them is to log what happened and just output a Non-variable
				switch definitionError.ErrStr {
				case errNoProcessor:
					Logger.Warnf("%s - %s", pos, derr)
					goto ignoreerror
				case errNotDefined:
					Logger.Debugf("%s - %s", pos, derr)
					goto ignoreerror
				case errNotDefinedInProcessor:
					Logger.Warnf("%s - %s", pos, derr)
					goto ignoreerror
				}
			}

			// many errors can occour here... for intance, the variable
			// does not exist, the processor doesn't exist, invalid input, ect.
			return totalBytes, derr

		ignoreerror:
			// Non-variables:
			// if we're here, then the variable didn't exist. So we've got to
			// print out the original contents (including '$(' and ')'). The
			// easiest way I see doing this is: just set the definer to
			// read from the variable read buffer. An elegant solution.
			// We'll re-use the NormalDefinition struct to do this. A very
			// elegant solution indeed.
			// To detect if def is a non-variable, just check if definitionError
			// is non-nil
			def = &NormalDefinition{value: pos.fullName}
		}
		// lets start reading it on the next read by setting c.currentlyReadingDef
		// to a non-nil value (see readDefinition)
		// We will also add it to the definition stack to detect for circular
		// definitions
		*c.definitionStack = append(*c.definitionStack, pos.fullName)

		// if its a normal variable, the definition up as a file so it can
		// read from other definitions.
		// we also have to make sure that it's a valid variable and not
		// just variable name defining itself (see Non-variables)
		if pos.processorName == "" && definitionError == nil {
			// but before we go on, lets make sure we are not running into
			// a recursively defining defintion.
			for i := 0; i < len(*c.definitionStack)-1; i++ {
				if (*c.definitionStack)[i] != pos.fullName {
					continue
				}
				// oh no. A parent definition is trying to define itself
				// right now, thats a recursive problem. Error out.
				oerr := NewError(errCircularDefinition)
				attemptedstack := append(*c.definitionStack, pos.fullName)
				oerr.SetSubjectf("%s", strings.Join(attemptedstack, " -> "))
				return totalBytes, oerr
			}
			c.currentlyReadingDef = &nonConvertedFile{
				sourceDocument:     c.sourceDocument,
				sourceFile:         def,
				variableReadBuffer: make([]byte, MaxVariableLength),
				definitionStack:    c.definitionStack,
			}
		} else {
			// it is a processor variable, do not allow nested variables to be
			// defined.
			c.currentlyReadingDef = def
		}

		// This next if statment is completely optional.
		// all this does is ask if there's any more space in dest we haven't
		// used. Without this if statment, the caller would just have to call
		// Read more often... so this ooptimizaiton is just for the caller's
		// convenience
		if len(dest) > nonVarByteCount {
			// lets use up the rest of the buffer we were given in /this/ call
			// to try to fill the /next/ one.
			secondCallBytesRead, err := c.Read(dest[nonVarByteCount:])
			totalBytes += secondCallBytesRead
			return totalBytes, err
		}
		return totalBytes, err
	}

	// at this point we know that a variable was not found, but not all bytes were
	// ignored. Which means we STARTED to scan a variable but we need to call
	// another read to grab the rest of it.
	return totalBytes, err
}

func (c *nonConvertedFile) Reset() error {
	//clear the variable buffer
	for i := 0; i < len(c.variableReadBuffer); i++ {
		c.variableReadBuffer[i] = 0
	}

	err := c.sourceFile.Reset()
	if err != nil {
		return err
	}

	c.bytesRead = 0
	return nil
}

func (c *nonConvertedFile) Close() error {
	if c.currentlyReadingDef != nil {
		_ = c.currentlyReadingDef.Close()
	}

	err := c.sourceFile.Close()
	if err != nil {
		return err
	}

	return nil
}
