// A friend path's final vals smart cast as the module's own: a module's
// unit tests read result.inspection.end after result.inspection!!.
package lib

class Inspection(val id: Int, val end: String?)

data class Result(val status: Int, val inspection: Inspection?)
