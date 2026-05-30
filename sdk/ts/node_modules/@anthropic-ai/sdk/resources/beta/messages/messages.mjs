// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.
import { APIResource } from "../../../resource.mjs";
import * as BatchesAPI from "./batches.mjs";
export class Messages extends APIResource {
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
(function (Messages) {
    Messages.Batches = BatchesAPI.Batches;
    Messages.BetaMessageBatchesPage = BatchesAPI.BetaMessageBatchesPage;
})(Messages || (Messages = {}));
//# sourceMappingURL=messages.mjs.map