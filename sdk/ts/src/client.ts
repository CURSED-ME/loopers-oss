import OpenAI from 'openai';
import Anthropic from '@anthropic-ai/sdk';

export interface LoopersClientOptions {
  loopersUrl: string;
  loopersKey: string;
  providerKey?: string;
  sessionId?: string;
  sessionBudget?: number;
  maxSteps?: number;
}

function createLoopersFetch(
  loopersKey: string,
  providerKey?: string,
  sessionId?: string,
  sessionBudget?: number,
  maxSteps?: number,
  customFetch?: typeof fetch
) {
  const originalFetch = customFetch || (typeof fetch !== 'undefined' ? fetch : undefined);
  if (!originalFetch) {
    throw new Error('A global fetch function is not available. Please pass a custom fetch implementation (e.g. node-fetch) or use Node.js 18+.');
  }

  return async (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
    // In JS/TS, headers can be in different structures
    const headers = new Headers(init?.headers || {});

    // Set Loopers headers
    headers.set('Authorization', `Bearer ${loopersKey}`);
    if (providerKey) {
      headers.set('X-Loopers-Provider-Key', providerKey);
    }
    if (sessionId) {
      headers.set('X-Loopers-Session-ID', sessionId);
    }
    if (sessionBudget !== undefined) {
      headers.set('X-Loopers-Session-Budget', String(sessionBudget));
    }
    if (maxSteps !== undefined) {
      headers.set('X-Loopers-Session-Max-Steps', String(maxSteps));
    }

    const modifiedInit = {
      ...init,
      headers,
    };

    const res = await originalFetch(input, modifiedInit);

    // Intercept response.json() to attach loopers properties to the returned parsed object
    const originalJson = res.json;
    res.json = async () => {
      const data = await originalJson.call(res);
      if (data && typeof data === 'object') {
        const costVal = res.headers.get('X-Loopers-Request-Cost');
        const estCostVal = res.headers.get('X-Loopers-Request-Cost-Estimated');
        const spendVal = res.headers.get('X-Loopers-Session-Spend');
        const stepsVal = res.headers.get('X-Loopers-Session-Steps');
        const remainingVal = res.headers.get('X-Loopers-Session-Remaining');

        Object.defineProperties(data, {
          loopers_cost: {
            value: costVal ? Number(costVal) : null,
            writable: true,
            enumerable: true,
          },
          loopers_cost_estimated: {
            value: estCostVal ? Number(estCostVal) : null,
            writable: true,
            enumerable: true,
          },
          loopers_session_spend: {
            value: spendVal ? Number(spendVal) : null,
            writable: true,
            enumerable: true,
          },
          loopers_session_steps: {
            value: stepsVal ? Number(stepsVal) : null,
            writable: true,
            enumerable: true,
          },
          loopers_session_remaining: {
            value: remainingVal ? Number(remainingVal) : null,
            writable: true,
            enumerable: true,
          },
        });
      }
      return data;
    };

    return res;
  };
}

export class LoopersOpenAI extends OpenAI {
  constructor(
    options: LoopersClientOptions & {
      [key: string]: any;
    }
  ) {
    const {
      loopersUrl,
      loopersKey,
      providerKey,
      sessionId,
      sessionBudget,
      maxSteps,
      _providerPath,
      ...openaiOptions
    } = options;

    const baseFetch = openaiOptions.fetch;
    const loopersFetch = createLoopersFetch(
      loopersKey,
      providerKey,
      sessionId,
      sessionBudget,
      maxSteps,
      baseFetch
    );

    super({
      ...openaiOptions,
      baseURL: `${loopersUrl.replace(/\/$/, '')}/${_providerPath || 'openai/v1'}`,
      apiKey: loopersKey,
      fetch: loopersFetch,
    });
  }
}

export class LoopersGroq extends LoopersOpenAI {
  constructor(options: LoopersClientOptions & { [key: string]: any }) {
    super({ ...options, _providerPath: 'groq/v1' });
  }
}

export class LoopersMistral extends LoopersOpenAI {
  constructor(options: LoopersClientOptions & { [key: string]: any }) {
    super({ ...options, _providerPath: 'mistral/v1' });
  }
}

export class LoopersDeepSeek extends LoopersOpenAI {
  constructor(options: LoopersClientOptions & { [key: string]: any }) {
    super({ ...options, _providerPath: 'deepseek/v1' });
  }
}

export class LoopersTogether extends LoopersOpenAI {
  constructor(options: LoopersClientOptions & { [key: string]: any }) {
    super({ ...options, _providerPath: 'together/v1' });
  }
}

export class LoopersAnthropic extends Anthropic {
  constructor(
    options: LoopersClientOptions & {
      [key: string]: any;
    }
  ) {
    const {
      loopersUrl,
      loopersKey,
      providerKey,
      sessionId,
      sessionBudget,
      maxSteps,
      ...anthropicOptions
    } = options;

    const baseFetch = anthropicOptions.fetch;
    const loopersFetch = createLoopersFetch(
      loopersKey,
      providerKey,
      sessionId,
      sessionBudget,
      maxSteps,
      baseFetch
    );

    super({
      ...anthropicOptions,
      baseURL: `${loopersUrl.replace(/\/$/, '')}/anthropic`,
      authToken: loopersKey,
      fetch: loopersFetch,
    });
  }
}
