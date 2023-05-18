# Store API Native Code

Here it lies the C++ code wrapping around the Windows Runtime APIs to provide both
the background agent as well as the graphical interface client.

The code organization aims to allow the language briding wrappers to be as thin as possible,
with very minimal logic except for translating input parameters and some memory allocation 
mechanisms required to allow crossing a specific ABI.

The code is not exported in any sense. Wrappers must pick and choose what they need and
import from source.
