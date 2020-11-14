package vorlage

// todo: I don't think this method should belong to Document...
// ARCHITECTUAL ERROR.
func (doc *Document) define(pos variablePos) (Definition, error) {
	var foundDef Definition

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
		// i  = the index of vars (array of pointers)
		vars := doc.compiler.processorInfos[pi].Variables
		var i int
		for i = 0; i < len(vars); i++ {
			if vars[i].Name == pos.processorVariableName {
				break
			}
		}
		if i == len(vars) {
			// we didn't find the variable in the processor
			oerr := NewError(errNotDefinedInProcessor)
			oerr.SetSubject(pos.String())
			return nil, oerr
		}

		// at this point: we've found the processor, we've foudn the variable
		// but what about the variable's inputs... let's make sure they're
		// populated.

		// static input
		for k := range vars[i].Input {
			if v, ok := doc.args.staticInputs[k]; ok {
				vars[i].Input[k] = v
			} else {
				// 0 if not given
				vars[i].Input[k] = ""
			}
		}

		// stream input
		for k := range vars[i].StreamedInput {
			if v, ok := doc.args.streamInputs[k]; ok {

				// mark it as used
				// or fail if it already was used.
				if err := doc.consumeInputStringOk(k, pos.fullName); err != nil {
					return nil, err
				}

				// now actually set the stream
				vars[i].StreamedInput[k] = v
			} else {
				// nil if input name not given
				vars[i].StreamedInput[k] = nil
			}
		}

		// lets recap, it's a processor variable. We found the processor.
		// we found the variable. we found all of it's inputs.
		// lets define it.
		foundDef = doc.compiler.processors[pi].DefineVariable(doc.request.Rid, vars[i])
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
	if pv, ok := doc.args.streamInputsUsed[streamName]; ok {
		// it was. That's an error.
		oerr := NewError(errDoubleInputStream)
		oerr.SetSubjectf("\"%s\" requested by %s but was used by %s already", streamName, requestor, pv)
		return oerr
	}
	// it was not. So lets keep track this this streamed input
	// was just consumed by this procvar.
	doc.args.streamInputsUsed[streamName] = requestor
	return nil
}
