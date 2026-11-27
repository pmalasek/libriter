// Adresa serveru – pravidla jsou sdílená s mobilem (core-shared/src/serverUrl.ts):
// zadané schéma platí přesně, místní adresa bez schématu zkusí https a pak
// http, veřejná jen https.
export { displayServer, hasServerScheme, isInsecure, isLocalHost, serverCandidates } from 'sceniter-shared'
