package doccomp

import "io"

func (c *nonConvertedFile) Read(dest []byte) (n int, err error) {
	if c.hasEOFd {
		return 0, io.EOF
	}

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

			dest = dest[n:]

			//return n, nil
		} else {
			// we're not done reading the definition yet.
			return n, nil
		}
	}

	// before we do another read from the file, we need to make sure that
	// tmpBuff has been emptied.
	if len(c.tmpBuff) != 0 {
		bytesCopied := copy(dest, c.tmpBuff)
		c.tmpBuff = c.tmpBuff[bytesCopied:]
		dest = dest[bytesCopied:]
		n += bytesCopied
		if len(c.tmpBuff) != 0 {
			// if we still have stuff in tmp buffer, we need another read.
			return n, nil
		}
	}

	// so if we're done reading the definition, move along with another read
	// from the file

	var sourceBytesRead int
	sourceBytesRead, err = c.sourceFile.Read(dest)

	// we WILL NOT append this bytes to n. Because if we do there's a chance
	// we've read in a variable. We don't want that to show up for the caller.
	//n+=sourceBytesRead
	if err != nil && err != io.EOF {
		return n, err
	}
	// set hasEOFd so future calls will return EOF.
	c.hasEOFd = err == io.EOF
	c.bytesRead += int64(sourceBytesRead)

	// do the draw parse into variable buffer from what we've read from the
	// source stream.
	nonVarByteCount, pos, cerr := drawParseVar(c.variableReadBuffer, dest[:sourceBytesRead], c.bytesRead)
	if cerr != nil {
		return n + nonVarByteCount, *cerr
	}
	if nonVarByteCount == sourceBytesRead {
		return n + nonVarByteCount, err
	}
	if pos != nil {
		// we have stumbled apon a variable.
		def, derr := c.sourceDocument.define(*pos)
		if derr != nil {
			return n + nonVarByteCount, derr
		}

		// lets start reading it on the next read.
		c.currentlyReadingDef = def

		// but first we ask:
		// did the buffer (dest) pick up anything after the variable?
		// (ie dest[:n] = "123$(varible)abc")
		//                    ^         ^  ^
		//                   (a)       (b)(c)
		//
		//  (a) = position of nonVarByteCount
		//  (b) = position of nonVarByteCount + len(pos.fullName)
		//  (c) = position of sourceBytesRead (length of string)
		// if so, we need to save the extra (ie "abc") to tmpBuff because
		// dest will be used to read-in the variable and will in turn all
		// content that was read after the variable from the file.
		if sourceBytesRead > nonVarByteCount+len(pos.fullName) {

			// calculate the remaining buffer length
			remainingBuffLen := sourceBytesRead - (nonVarByteCount + len(pos.fullName))
			// make the tmp buff the size of everything after the variable.
			c.tmpBuff = make([]byte, remainingBuffLen)
			// copy everything after that variable into that buffer.
			copy(c.tmpBuff, dest[nonVarByteCount+len(pos.fullName):sourceBytesRead])

			// lets use up the rest of the buffer we were given in this call
			// to try to fill the next one. You could comment these three
			// lines out and the only thing it would really affect is the
			// fact that the caller needs to call Read a few more times.
			//lmlog.AlertF("%s", string(dest[nonVarByteCount:]))
			bytesOfDefinition, err := c.Read(dest[nonVarByteCount:])
			n += bytesOfDefinition + nonVarByteCount
			return n, err
		}
		return n + nonVarByteCount, err
	}

	// at this point we know that a variable was not found, but not all bytes were
	// ignored.
	return n + nonVarByteCount, err
}

func (c *nonConvertedFile) Rewind() error {
	//clear the variable buffer
	for i := 0; i < len(c.variableReadBuffer); i++ {
		c.variableReadBuffer[i] = 0
	}

	err := c.sourceFile.Rewind()
	if err != nil {
		return err
	}
	c.hasEOFd = false

	c.bytesRead = 0
	return nil
}

func (c *nonConvertedFile) Close() error {
	if c.currentlyReadingDef != nil {
		_ = c.currentlyReadingDef.Reset()
	}

	err := c.sourceFile.Close()
	if err != nil {
		return err
	}

	return nil
}
