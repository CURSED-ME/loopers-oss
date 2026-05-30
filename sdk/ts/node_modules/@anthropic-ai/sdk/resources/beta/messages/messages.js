"use strict";
// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.
var __createBinding = (this && this.__createBinding) || (Object.create ? (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    var desc = Object.getOwnPropertyDescriptor(m, k);
    if (!desc || ("get" in desc ? !m.__esModule : desc.writable || desc.configurable)) {
      desc = { enumerable: true, get: function() { return m[k]; } };
    }
    Object.defineProperty(o, k2, desc);
}) : (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    o[k2] = m[k];
}));
var __setModuleDefault = (this && this.__setModuleDefault) || (Object.create ? (function(o, v) {
    Object.defineProperty(o, "default", { enumerable: true, value: v });
}) : function(o, v) {
    o["default"] = v;
});
var __importStar = (this && this.__importStar) || function (mod) {
    if (mod && mod.__esModule) return mod;
    var result = {};
    if (mod != null) for (var k in mod) if (k !== "default" && Object.prototype.hasOwnProperty.call(mod, k)) __createBinding(result, mod, k);
    __setModuleDefault(result, mod);
    return result;
};
Object.defineProperty(exports, "__esModule", { value: true });
exports.Messages = void 0;
const resource_1 = require("../../../resource.js");
const BatchesAPI = __importStar(require("./batches.js"));
class Messages extends resource_1.APIResource {
    constructor() {
        super(...arguments);
        this.batches = new BatchesAPI.Batches(this._client);
    }
    create(params, options) {
        const { betas, ...body } = params;
        return this._client.post('/v1/messages?beta=true', {
            body,
            timeout: this._client._options.timeout ?? 600000,
            ...options,
            headers: {
                ...(betas?.toString() != null ? { 'anthropic-beta': betas?.toString() } : undefined),
                ...options?.headers,
            },
            stream: params.stream ?? false,
        });
    }
}
exports.Messages = Messages;
(function (Messages) {
    Messages.Batches = BatchesAPI.Batches;
    Messages.BetaMessageBatchesPage = BatchesAPI.BetaMessageBatchesPage;
})(Messages = exports.Messages || (exports.Messages = {}));
//# sourceMappingURL=messages.js.map