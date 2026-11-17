// JXA uses macOS's built-in JSON parser, including Apple's nullable issues field.
function run(argv) {
    ObjC.import("Foundation");
    const source = $.NSString.stringWithContentsOfFileEncodingError(argv[0], $.NSUTF8StringEncoding, null);
    const log = JSON.parse(ObjC.unwrap(source));
    if (log.status !== "Accepted" || (log.issues !== null && (!Array.isArray(log.issues) || log.issues.length !== 0))) {
        throw new Error("Notarization was not accepted without issues; inspect the notary log.");
    }
}
