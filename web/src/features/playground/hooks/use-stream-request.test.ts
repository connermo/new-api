/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { parseStreamChunk } from '../lib'
import type { ChatCompletionRequest } from '../types'
import { createStreamRequestController } from './use-stream-request'

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((promiseResolve) => {
    resolve = promiseResolve
  })
  return { promise, resolve }
}

class FakeStreamSource {
  readyState = 0
  closed = false
  streamed = false
  private listeners = new Map<
    string,
    Array<(event: Event & { data?: string; readyState?: number }) => void>
  >()

  addEventListener(
    type: string,
    listener: (event: Event & { data?: string; readyState?: number }) => void
  ) {
    const listeners = this.listeners.get(type) ?? []
    listeners.push(listener)
    this.listeners.set(type, listeners)
  }

  close() {
    this.closed = true
  }

  stream() {
    this.streamed = true
  }

  emit(type: string, data?: string) {
    for (const listener of this.listeners.get(type) ?? []) {
      listener({ data, readyState: this.readyState } as Event & {
        data?: string
        readyState?: number
      })
    }
  }
}

const payload: ChatCompletionRequest = {
  model: 'test-model',
  messages: [{ role: 'user', content: 'hello' }],
  stream: true,
}

const noopCallbacks = {
  onUpdate: () => undefined,
  onComplete: () => undefined,
  onError: () => undefined,
}

describe('latest-wins stream request coordination', () => {
  test('only creates a stream for the latest header request', async () => {
    const firstHeaders = deferred<Record<string, string>>()
    const secondHeaders = deferred<Record<string, string>>()
    let headerRequest = 0
    const sources: FakeStreamSource[] = []
    const controller = createStreamRequestController({
      getHeaders: () => {
        headerRequest += 1
        return headerRequest === 1
          ? firstHeaders.promise
          : secondHeaders.promise
      },
      createSource: () => {
        const source = new FakeStreamSource()
        sources.push(source)
        return source
      },
      setStreaming: () => undefined,
    })

    const first = controller.send(payload, noopCallbacks)
    const second = controller.send(payload, noopCallbacks)
    firstHeaders.resolve({ Authorization: 'Bearer stale' })
    await first
    assert.equal(sources.length, 0)

    secondHeaders.resolve({ Authorization: 'Bearer current' })
    await second
    assert.equal(sources.length, 1)
    assert.equal(sources[0]?.streamed, true)
  })

  test('stop cancels a request that is still waiting for headers', async () => {
    const headers = deferred<Record<string, string>>()
    let sourceCount = 0
    const controller = createStreamRequestController({
      getHeaders: () => headers.promise,
      createSource: () => {
        sourceCount += 1
        return new FakeStreamSource()
      },
      setStreaming: () => undefined,
    })

    const request = controller.send(payload, noopCallbacks)
    controller.stop()
    headers.resolve({ Authorization: 'Bearer ignored' })
    await request

    assert.equal(sourceCount, 0)
  })

  test('dispose cancels a pending header request without a state update', async () => {
    const headers = deferred<Record<string, string>>()
    const streamingStates: boolean[] = []
    let sourceCount = 0
    const controller = createStreamRequestController({
      getHeaders: () => headers.promise,
      createSource: () => {
        sourceCount += 1
        return new FakeStreamSource()
      },
      setStreaming: (streaming) => streamingStates.push(streaming),
    })

    const request = controller.send(payload, noopCallbacks)
    controller.dispose()
    headers.resolve({ Authorization: 'Bearer ignored' })
    await request

    assert.equal(sourceCount, 0)
    assert.deepEqual(streamingStates, [false])
  })

  test('closes the previous source and ignores all of its later events', async () => {
    const nextHeaders = deferred<Record<string, string>>()
    let headerRequest = 0
    const sources: FakeStreamSource[] = []
    const updates: string[] = []
    const controller = createStreamRequestController({
      getHeaders: () => {
        headerRequest += 1
        if (headerRequest === 1) {
          return Promise.resolve({ Authorization: 'Bearer first' })
        }
        return nextHeaders.promise
      },
      createSource: () => {
        const source = new FakeStreamSource()
        sources.push(source)
        return source
      },
      setStreaming: () => undefined,
    })
    const callbacks = {
      onUpdate: (_type: 'reasoning' | 'content', chunk: string) =>
        updates.push(chunk),
      onComplete: () => undefined,
      onError: () => undefined,
    }

    await controller.send(payload, callbacks)
    const second = controller.send(payload, callbacks)
    assert.equal(sources[0]?.closed, true)
    sources[0]?.emit(
      'message',
      JSON.stringify({ choices: [{ delta: { content: 'stale' } }] })
    )

    nextHeaders.resolve({ Authorization: 'Bearer second' })
    await second
    sources[1]?.emit(
      'message',
      JSON.stringify({ choices: [{ delta: { content: 'current' } }] })
    )

    assert.deepEqual(updates, ['current'])
  })
})

describe('stream chunk usage parsing', () => {
  test('reads usage from the trailing chunk that carries no choices', () => {
    // Upstreams report token counts on a final chunk with an empty choices
    // array, so usage must be read independently of the delta.
    const result = parseStreamChunk(
      JSON.stringify({
        choices: [],
        usage: { prompt_tokens: 11, completion_tokens: 7, total_tokens: 18 },
      })
    )

    assert.deepEqual(result.updates, [])
    assert.deepEqual(result.usage, {
      prompt_tokens: 11,
      completion_tokens: 7,
      total_tokens: 18,
    })
  })

  test('returns content and reasoning updates when no usage is present', () => {
    const result = parseStreamChunk(
      JSON.stringify({
        choices: [{ delta: { reasoning_content: 'think', content: 'hi' } }],
      })
    )

    assert.deepEqual(result.updates, [
      { type: 'reasoning', chunk: 'think' },
      { type: 'content', chunk: 'hi' },
    ])
    assert.equal(result.usage, undefined)
  })

  test('keeps usage attached to a chunk that also carries a delta', () => {
    const result = parseStreamChunk(
      JSON.stringify({
        choices: [{ delta: { content: 'done' } }],
        usage: { prompt_tokens: 3, completion_tokens: 4, total_tokens: 7 },
      })
    )

    assert.deepEqual(result.updates, [{ type: 'content', chunk: 'done' }])
    assert.deepEqual(result.usage, {
      prompt_tokens: 3,
      completion_tokens: 4,
      total_tokens: 7,
    })
  })
})
