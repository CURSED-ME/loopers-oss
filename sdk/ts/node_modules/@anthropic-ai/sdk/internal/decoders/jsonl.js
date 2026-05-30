"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.JSONLDecoder = void 0;
const error_1 = require("../../error.js");
const streaming_1 = require("../../streaming.js");
const line_1 = require("./line.js");
class JSONLDecoder {
    constructor(iterator, controller) {
        this.iterator = iterator;
        this.controller = controller;
    }
    async *decoder() {
        const lineDecoder = new line_1.LineDecoder();
        for await (const chunk of this.iterator) {
            for (const line of lineDecoder.decode(chunk)) {
                yield JSON.parse(line);
            }
        }
        for (const line of lineDecoder.flush()) {
            yield JSON.parse(line);
        }
    }
    [Symbol.asyncIterator]() {
        return this.decoder();
    }
    static fromResponse(response, controller) {
        if (!response.body) {
            controller.abort();
            throw new error_1.AnthropicError(`Attempted to iterate over a response with no body`);
        }
        return new JSONLDecoder((0, streaming_1.readableStreamAsyncIterable)(response.body), controller);
    }
}
exports.JSONLDecoder = JSONLDecoder;
//# sourceMappingURL=jsonl.js.map