import OpenAI from 'openai';
import * as jose from 'jose';
import Anthropic from '@anthropic-ai/sdk';
import { LoopersPolicyDenied, parsePolicyDenial, formatAsToolOutput } from './policyError';

// Re-export policy error types for consumers who import from this module
export { LoopersPolicyDenied, parsePolicyDenial, formatAsToolOutput };
export type { PolicyDeniedDetails } from './policyError';

export interface LoopersClientOptions {
  loopersUrl: string;
  loopersKey: string;
  providerKey?: string;
  sessionId?: string;
  sessionBudget?: number;
  maxSteps?: number;
  sessionTtl?: number;
  maxTools?: number;
  maxServers?: number;
  /**
   * Optional callback invoked when a request is blocked by a Loopers policy.
   *
   * When provided, the SDK will call this instead of throwing a LoopersPolicyDenied error.
   * The callback receives the denial object and the original fetch Response.
   * Return a replacement Response to substitute for the blocked call (e.g., a mock
   * tool-output error string), or undefined to let the SDK throw.
   *
   * @example
   * ```ts
   * onPolicyBlock: (denial, _res) => {
   *   console.warn(`Policy block: ${denial.message}`);
   *   // Return a fake "tool error" response so the LLM can self-correct
   *   const body = JSON.stringify({ error: denial.message });
   *   return new Response(body, { status: 200 });
   * }
   * ```
   */
  onPolicyBlock?: (denial: LoopersPolicyDenied, originalResponse: Response) => Response | undefined | void;
}

function createLoopersFetch(
  loopersUrl: string,
  loopersKey: string,
  providerKey?: string,
  sessionId?: string,
  sessionBudget?: number,
  maxSteps?: number,
  sessionTtl?: number,
  maxTools?: number,
  maxServers?: number,
  customFetch?: typeof fetch,
  onPolicyBlock?: LoopersClientOptions['onPolicyBlock']
) {

  let ephemeralKey: any;
  let dpopJwkPublic: any;
  let zspToken: string | null = null;
  let tokenExpiresAt = 0;
  let bootstrapPromise: Promise<void> | null = null;
  const isCloud = loopersKey.startsWith('lc_');

  const originalFetch = customFetch || (typeof fetch !== 'undefined' ? fetch : undefined);
  if (!originalFetch) {
    throw new Error('A global fetch function is not available. Please pass a custom fetch implementation (e.g. node-fetch) or use Node.js 18+.');
  }

  return async (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
    // In JS/TS, headers can be in different structures
    const headers = new Headers(init?.headers || {});

    // Set Loopers headers
    // ZSP Logic
    if (isCloud) {
      if (Date.now() >= tokenExpiresAt) {
        if (!bootstrapPromise) {
          bootstrapPromise = (async () => {
            try {
              const { publicKey, privateKey } = await jose.generateKeyPair('ES256', { extractable: true });
              ephemeralKey = privateKey;
              dpopJwkPublic = await jose.exportJWK(publicKey);
              
              const res = await originalFetch(`${loopersUrl.replace(/\/$/, '')}/v1/auth/token`, {
                method: 'POST',
                headers: {
                  'Authorization': `Bearer ${loopersKey}`,
                  'Content-Type': 'application/json'
                },
                body: JSON.stringify({ dpop_jwk: dpopJwkPublic })
              });
              if (!res.ok) throw new Error('ZSP bootstrap failed: ' + res.statusText);
              const data = await res.json();
              zspToken = data.access_token;
              tokenExpiresAt = Date.now() + (data.expires_in * 1000) - (300 * 1000);
            } finally {
              bootstrapPromise = null;
            }
          })();
        }
        await bootstrapPromise;
      }
      headers.set('Authorization', `Bearer ${zspToken}`);

      const method = (init?.method || 'GET').toUpperCase();
      let urlStr = '';
      if (typeof input === 'string') {
        urlStr = input;
      } else if (input instanceof URL) {
        urlStr = input.toString();
      } else if (input && (input as any).url) {
        urlStr = (input as any).url;
      }
      
      const parsedUrl = new URL(urlStr);
      const htu = `${parsedUrl.protocol}//${parsedUrl.host}${parsedUrl.pathname}`;
      
      // Fallback for crypto.randomUUID
      let jti = '';
      if (typeof crypto !== 'undefined' && crypto.randomUUID) {
        jti = crypto.randomUUID();
      } else {
        jti = Math.random().toString(36).substring(2) + Date.now().toString(36);
      }

      const dpopProof = await new jose.SignJWT({
        jti,
        htm: method,
        htu,
      })
        .setProtectedHeader({ typ: 'dpop+jwt', alg: 'ES256', jwk: dpopJwkPublic })
        .setIssuedAt()
        .sign(ephemeralKey);
        
      headers.set('DPoP', dpopProof);
    } else {
      headers.set('Authorization', `Bearer ${loopersKey}`);
    }

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
    if (sessionTtl !== undefined) {
      headers.set('X-Loopers-Session-TTL', String(sessionTtl));
    }
    if (maxTools !== undefined) {
      headers.set('X-Loopers-Session-Max-Tools', String(maxTools));
    }
    if (maxServers !== undefined) {
      headers.set('X-Loopers-Session-Max-Servers', String(maxServers));
    }

    const modifiedInit = {
      ...init,
      headers,
    };

    const res = await originalFetch(input, modifiedInit);

    // --- Policy Denial Interception ---
    // Detect policy blocks (HTTP 403 OR HTTP 200 with X-Loopers-Policy-Block header).
    // Convert them to LoopersPolicyDenied rather than letting agent frameworks
    // crash on a raw HTTP exception.
    const isPolicyBlock =
      res.status === 403 ||
      res.headers.get('X-Loopers-Policy-Block') === 'true';

    if (isPolicyBlock) {
      // Clone to safely read body without consuming the original
      const cloned = res.clone();
      try {
        const data = await cloned.json();
        const denial = parsePolicyDenial(data, undefined, sessionId);
        if (denial) {
          if (onPolicyBlock) {
            const replacement = onPolicyBlock(denial, res);
            if (replacement) return replacement;
          }
          throw denial;
        }
      } catch (e) {
        // Re-throw LoopersPolicyDenied directly; ignore JSON parse errors on non-policy 403s
        if (e instanceof LoopersPolicyDenied) throw e;
      }
    }

    // Intercept response.json() to attach loopers properties to the returned parsed object
    const originalJson = res.json;
    res.json = async () => {
      const data = await originalJson.call(res);
      if (data && typeof data === 'object') {
        const costVal = res.headers.get('X-Loopers-Request-Cost');
        const estCostVal = res.headers.get('X-Loopers-Request-Cost-Estimated');
        const stepsVal = res.headers.get('X-Loopers-Session-Steps');

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
          loopers_session_steps: {
            value: stepsVal ? Number(stepsVal) : null,
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
      sessionTtl,
      maxTools,
      maxServers,
      onPolicyBlock,
      _providerPath,
      ...openaiOptions
    } = options;

    const baseFetch = openaiOptions.fetch;
    const loopersFetch = createLoopersFetch(
      loopersUrl,
      loopersKey,
      providerKey,
      sessionId,
      sessionBudget,
      maxSteps,
      sessionTtl,
      maxTools,
      maxServers,
      baseFetch,
      onPolicyBlock
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
      sessionTtl,
      maxTools,
      maxServers,
      onPolicyBlock,
      ...anthropicOptions
    } = options;

    const baseFetch = anthropicOptions.fetch;
    const loopersFetch = createLoopersFetch(
      loopersUrl,
      loopersKey,
      providerKey,
      sessionId,
      sessionBudget,
      maxSteps,
      sessionTtl,
      maxTools,
      maxServers,
      baseFetch,
      onPolicyBlock
    );

    super({
      ...anthropicOptions,
      baseURL: `${loopersUrl.replace(/\/$/, '')}/anthropic`,
      authToken: loopersKey,
      fetch: loopersFetch,
    });
  }
}
