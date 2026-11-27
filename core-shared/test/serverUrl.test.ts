// Testy adresy serveru (core-shared/src/serverUrl.ts).
//
//   npm test -w core-shared

import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import {
  displayServer,
  isInsecure,
  isLocalHost,
  normalizeServerInput,
  serverCandidates,
  serverHost,
} from '../src/serverUrl'

describe('normalizeServerInput', () => {
  it('ořízne mezery, lomítka a cestu k API', () => {
    assert.equal(normalizeServerInput('  video.example.cz/  '), 'video.example.cz')
    assert.equal(normalizeServerInput('https://video.example.cz/api/v1/'), 'https://video.example.cz')
    assert.equal(normalizeServerInput('nas:8081/api/v1'), 'nas:8081')
    assert.equal(normalizeServerInput('   '), '')
  })
})

describe('serverHost', () => {
  it('vytáhne hostitele z různých zápisů', () => {
    assert.equal(serverHost('Video.Example.cz'), 'video.example.cz')
    assert.equal(serverHost('https://video.example.cz:8443/cesta?x=1'), 'video.example.cz')
    assert.equal(serverHost('192.168.1.5:8081'), '192.168.1.5')
    assert.equal(serverHost('[fe80::1%25eth0]:8081'), 'fe80::1')
    assert.equal(serverHost('http://[::1]:8081/'), '::1')
    assert.equal(serverHost('fd00::5'), 'fd00::5')
    assert.equal(serverHost('user:pass@nas.lan:8081'), 'nas.lan')
    assert.equal(serverHost('nas.local.'), 'nas.local')
  })
})

describe('isLocalHost', () => {
  const local = [
    '10.0.0.1',
    '10.255.255.255:8081',
    '172.16.0.1',
    '172.31.255.254',
    '192.168.1.5',
    '192.168.1.5:8081',
    '127.0.0.1',
    '169.254.10.20',
    '100.64.0.1',
    '100.127.255.255',
    '::1',
    '[::1]:8081',
    '0:0:0:0:0:0:0:1',
    'fc00::1',
    'fd12:3456:789a::1',
    '[fd12::1]:8081',
    'fe80::1',
    'febf::1',
    '[fe80::1%25en0]:8081',
    '::ffff:192.168.1.5',
    'localhost',
    'localhost:8081',
    'app.localhost',
    'nas.local',
    'nas.lan',
    'nas.home.arpa',
    'video.internal',
    'NAS',
    'synology:5000',
    'http://nas.lan:8081',
    'https://192.168.0.10',
  ]
  for (const host of local) {
    it(`místní: ${host}`, () => assert.equal(isLocalHost(host), true))
  }

  const remote = [
    'video.example.cz',
    'video.example.cz:8443',
    'https://video.example.cz',
    '8.8.8.8',
    '172.15.0.1',
    '172.32.0.1',
    '192.169.0.1',
    '100.63.255.255',
    '100.128.0.1',
    '11.0.0.1',
    '2001:db8::1',
    '[2a00:1450:4014::200e]:443',
    'fec0::1',
    '::ffff:8.8.8.8',
    'local.example.com',
    'example.lan.cz',
    '999.168.1.1',
    '',
  ]
  for (const host of remote) {
    it(`veřejná: ${JSON.stringify(host)}`, () => assert.equal(isLocalHost(host), false))
  }
})

describe('serverCandidates', () => {
  it('zadané schéma platí přesně', () => {
    assert.deepEqual(serverCandidates('http://video.example.cz/'), ['http://video.example.cz'])
    assert.deepEqual(serverCandidates('https://nas.lan:8081'), ['https://nas.lan:8081'])
    assert.deepEqual(serverCandidates('HTTP://192.168.1.5:8081'), ['HTTP://192.168.1.5:8081'])
  })

  it('místní adresa bez schématu zkusí https, pak http', () => {
    assert.deepEqual(serverCandidates('192.168.1.5:8081'), ['https://192.168.1.5:8081', 'http://192.168.1.5:8081'])
    assert.deepEqual(serverCandidates('nas'), ['https://nas', 'http://nas'])
    assert.deepEqual(serverCandidates('[fd00::5]:8081'), ['https://[fd00::5]:8081', 'http://[fd00::5]:8081'])
  })

  it('holou IPv6 adresu dá do závorek', () => {
    assert.deepEqual(serverCandidates('fd00::5'), ['https://[fd00::5]', 'http://[fd00::5]'])
    assert.deepEqual(serverCandidates('2001:db8::1'), ['https://[2001:db8::1]'])
  })

  it('veřejná adresa bez schématu jen https', () => {
    assert.deepEqual(serverCandidates('video.example.cz'), ['https://video.example.cz'])
    assert.deepEqual(serverCandidates(' video.example.cz/api/v1 '), ['https://video.example.cz'])
    assert.deepEqual(serverCandidates('8.8.8.8:8081'), ['https://8.8.8.8:8081'])
  })

  it('prázdný vstup nedá nic', () => {
    assert.deepEqual(serverCandidates(''), [])
    assert.deepEqual(serverCandidates(' / '), [])
  })
})

describe('isInsecure a displayServer', () => {
  it('http je nešifrované, https ne', () => {
    assert.equal(isInsecure('http://nas.lan'), true)
    assert.equal(isInsecure(' HTTP://nas.lan'), true)
    assert.equal(isInsecure('https://nas.lan'), false)
    assert.equal(isInsecure('nas.lan'), false)
  })

  it('zobrazení schová jen https', () => {
    assert.equal(displayServer('https://video.example.cz'), 'video.example.cz')
    assert.equal(displayServer('http://nas.lan:8081'), 'http://nas.lan:8081')
  })
})
