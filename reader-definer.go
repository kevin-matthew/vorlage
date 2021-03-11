package vorlage

import vorlageproc "ellem.so/vorlageproc"

// todo: I don't think this method should belong to Document...
// ARCHITECTUAL ERROR.
func (doc *Document) define(pos variablePos) (vorlageproc.Definition, error) {
	var foundDef vorlageproc.Definition

	// we have found a variable in the document.
	// lets go find it's definition

	// first we ask if its a processor variable or a normal variable?
	if len(pos.processorName) != 0 {
		// its a processed variable.
		// lets find the right processor...
		var pi int
		for pi = range doc.compiler.processorInfos {
			if doc.compiler.processorInfos[pi].Name == pos.processorName {
				break
			}
		}
		if pi == len(doc.compiler.processorInfos) {
			// processor not found
			oerr := NewError(errNoProcessor)
			oerr.SetSubject(pos.String())
			return nil, oerr
		}

		// at this point we've found the processor now we need to get
		// its variables to find the right one.
		// pi = the index of processorInfos that matches
		// procvarIndex  = the index of vars (array of pointers)
		vars := doc.compiler.processorInfos[pi].Variables
		var procvarIndex int
		for procvarIndex = 0; procvarIndex < len(vars); procvarIndex++ {
			if vars[procvarIndex].Name == pos.processorVariableName {
				break
			}
		}
		if procvarIndex == len(vars) {
			// we didn't find the variable in the processor
			oerr := NewError(errNotDefinedInProcessor)
			oerr.SetSubject(pos.String())
			return nil, oerr
		}

		// at this point: we've found the processor, we've foudn the variable
		// but what about the variable's inputs... let's make sure they're
		// populated.

		df := vorlageproc.DefineInfo{
			RequestInfo:  &doc.compRequest.processorRInfos[pi],
			ProcVarIndex: procvarIndex,
			Input:        make([]string, len(vars[procvarIndex].InputProto)),
			StreamInput:  make([]vorlageproc.StreamInput, len(vars[procvarIndex].StreamInputProto)),
		}

		// static input
		for k := range df.Input {
			name := vars[procvarIndex].InputProto[k].Name
			if v, ok := doc.compRequest.allInput[name]; ok {
				df.Input[k] = v
			} else {
				// 0 if not given
				Logger.Debugf("variable %s was not given %s input", pos.String(), name)
				df.Input[k] = ""
			}
		}

		// stream input
		for k := range df.StreamInput {
			name := vars[procvarIndex].StreamInputProto[k].Name
			if v, ok := doc.compRequest.allStreams[name]; ok {

				// mark it as used
				// or fail if it already was used.
				if err := doc.consumeInputStringOk(name, pos.fullName); err != nil {
					return nil, err
				}

				// now actually set the stream
				df.StreamInput[k] = v
			} else {
				// nil if input Name not given
				Logger.Debugf("variable %s was not given %s stream input", pos.String(), name)
				df.StreamInput[k] = nil
			}
		}

		// lets recap, it's a processor variable. We found the processor.
		// we found the variable. we found all of it's inputs.
		// lets define it.
		foundDef = doc.compiler.processors[pi].DefineVariable(df, *df.RequestInfo.Cookie)
	} else {
		// its a normal variable. Easy.
		// look through all the doucment's normal definitions.
		for i, d := range *(doc.allDefinitions) {
			if d.GetFullName() == pos.fullName {
				foundDef = &((*(doc.allDefinitions))[i])
				break
			}
		}
	}

	// did we find a definition from the logic above?
	if foundDef == nil {
		// we did not find the definition
		oerr := NewError(errNotDefined)
		oerr.SetSubject(pos.fullName)
		return foundDef, oerr
	}

	// found it!
	// lets start reading this normal definition.
	// but first we must reset it as per the Definition specification.
	err := foundDef.Reset()
	if err != nil {
		oerr := NewError(errResetVariable)
		oerr.SetSubject(pos.fullName)
		oerr.SetBecause(NewError(err.Error()))
		return nil, oerr
	}
	// okay that's out of the way. The next call will begin reading
	// the definition's contents.
	return foundDef, nil
}

// helper func to doc.define
// checks to see if input stream was already used. If so, error is returned.
// if not, marks the input stream as read.
// assumes stremName exists.
func (doc *Document) consumeInputStringOk(streamName string, requestor string) error {
	// so we found the stream this input wants... but was it
	// used by a previous procvar?
	if pv, ok := (doc.streamInputsUsed[streamName]); ok {
		// it was. That's an error.
		oerr := NewError(errDoubleInputStream)
		oerr.SetSubjectf("\"%s\" requested by %s but was used by %s already", streamName, requestor, pv)
		return oerr
	}
	// it was not. So lets keep track this this streamed input
	// was just consumed by this procvar.
	doc.streamInputsUsed[streamName] = requestor
	return nil
}
