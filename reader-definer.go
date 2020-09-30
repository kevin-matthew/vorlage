package doccomp

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
		p, ok := Processors[pos.processorName]
		if !ok {
			// processor not found
			oerr := NewError(errNoProcessor)
			oerr.SetSubject(pos.processorName)
			return nil, oerr
		}

		// at this point we've found the processor now we need to get
		// its variables to find the right one.
		vars := p.GetVariables()
		var i int
		for i = range vars {
			if vars[i].Name == pos.processorVariableName {
				break
			}
		}
		if i == len(vars) {
			// we didn't find it in the processor.
			oerr := NewError(errNotDefined)
			oerr.SetSubject(pos.fullName)
			return nil, oerr
		}
		// at this point: we've found the processor, we've foudn the variable
		// but what about the variable's inputs... do we have everything we
		// we need?
		for _, n := range vars[i].InputNames {
			// Now we ask ourselves (doc) if we've been given all the right
			// inputs
			_, foundStatic := doc.args.staticInputs[n]
			_, foundStream := doc.args.streamInputs[n]
			if !foundStatic && !foundStream {
				// op. we wern't given all the right inputs.
				oerr := NewError(errInputNotProvided)
				oerr.SetSubjectf("\"%s\" not provided for %s", n, pos.fullName)
				return nil, oerr
			}
			if foundStatic && foundStream {
				// for some reason we have both a static and stream input with
				// the same Name that are being requested. That's an error.
				oerr := NewError(errInputInStreamAndStatic)
				oerr.SetSubjectf("\"%s\" in %s", n, pos.fullName)
				return nil, oerr
			}
			if foundStream {
				// so we found the stream this input wants... but was it
				// used by a previous procvar?
				if pv, ok := doc.args.streamInputsUsed[n]; ok {
					// it was. That's an error.
					oerr := NewError(errDoubleInputStream)
					oerr.SetSubjectf("\"%s\" requested by %s but was used by %s already", n, pv, pos.fullName)
					return nil, oerr
				}
				// it was not. So lets keep track this this streamed input
				// was just consumed by this procvar.
				doc.args.streamInputsUsed[n] = pos.fullName
			}
			if foundStatic {
				// no further action needs to take place if we found the
				// static variable.
			}
		}

		// lets recap, it's a processor variable. We found the processor.
		// we found the variable. we found all of it's inputs.
		// lets define it.
		var logerr error
		foundDef, logerr = p.DefineVariable(pos.processorVariableName,
			doc.args.staticInputs,
			doc.args.streamInputs)

		// as per the documentation, if there's an error with the definition,
		// it is ignored. All proc vars MUST be defined as long as they're loaded.
		if logerr != nil {
			logger.Errorf("error defining %s: %s", pos.fullName, logerr.Error())
		}
	} else {
		// its a normal variable.
		// look through all the doucment's normal definitions.
		for i, d := range *(doc.allDefinitions) {
			if d.GetFullName() == pos.fullName {
				foundDef = &((*(doc.allDefinitions))[i])
				break
			}
		}
	}

	// did we find a definition from the logic above?
	if foundDef != nil {
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

	// we did not find the definition
	oerr := NewError(errNotDefined)
	oerr.SetSubject(pos.fullName)
	return foundDef, oerr
}
