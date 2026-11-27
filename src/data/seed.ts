import type { Highlight, Item, Source, UserItemState } from '../types/model.ts'
import type { Library } from '../lib/select.ts'

const QUOTE_CACHE =
  'A cache hit is not a performance detail. It is a promise you made to the reader about how fast the next sentence arrives.'
const QUOTE_QUEUE =
  'If the queue and the read archive share a flag, you cannot tell a skipped headline from a finished essay.'
const QUOTE_TRACE =
  'A trace that names the tool, the input, and the sentence it changed is worth more than another leaderboard row.'

const latencyMd = `A reader does not experience your architecture. They experience the gap between intending to read the next sentence and seeing it. That gap is the latency budget, and it is smaller than most product reviews admit.

I started keeping a budget the way a film editor keeps a frame count. Two hundred milliseconds to paint the list. Fifty milliseconds to open a piece I have already stored. Zero network calls on the path that marks something read, saved, or dismissed. If a number does not fit, the feature does not ship until it does.

The expensive mistakes were never the slow queries. They were the states I had fused together. ${QUOTE_CACHE} Treat the budget as a contract with the person, not as a graph in a profiler.

Queues are where the contract gets muddy. People skip. People finish. People park a long essay for a Sunday. ${QUOTE_QUEUE} Search then lies, and the archive fills with things the reader never wanted to remember.

I keep four places, and only one of them means I read this. The firehose is arrival. Later is a promise to my future self. Dismissed is a closed door that does not pretend to be a memory. Read is the archive, and it is the only set search should trust.

Half-read is not a fifth place. It is a measure. If I stop partway through an essay in Later, the essay stays in Later. The scroll position is a pencil mark, not a verdict. I should be able to come back, see the mark, and continue without the app congratulating me for a job I did not finish.

> Dismiss is a closed door. Read is a memory. They are not the same door.

Local storage is enough to learn this. The model is a handful of records: a source, an item, a state, a highlight. The state row is doing the real work. Queue is orthogonal to \`readAt\`. If you let a single enum carry both ideas, you will spend the next year writing migrations that are really apologies.

A parked essay, the kind sitting in Later with a pencil mark, is this row:

\`\`\`
queue: later
readAt: null
readProgress: 0.35
\`\`\`

Highlights have a similar shape. A quote, a prefix, a suffix, a note. The note is not a comment thread. It sits in the margin, beside the sentence, the way a pencil sits in a book you own. On a narrow screen the margin disappears and the note tucks under the paragraph. The words do not change, only the furniture.

There is a temptation to add a badge for every state, a pill for every source, a banner for every empty morning. Resist it. The list is the interface. A title, an author, a date, and the few actions a thumb can hit without looking. If a control needs a paragraph of explanation, it is the wrong control.

I measure the budget on a train with a cold phone. Open the app. The list is there. Open the essay. The sentence I left is there, and so is the note I wrote in the margin. Mark it read. It leaves the queue and enters the archive. Dismiss the next one. It leaves the queue and does not enter the archive. The difference is the product.

When the budget holds, the reading feels uneventful. That is the point. Speed is not a feature you announce. It is the absence of a reason to leave.`

const cachesMd = `A cache is a product decision wearing an infrastructure costume. You are choosing which version of the world a person is allowed to see, and how long they are allowed to see it.

## The haunted list

I learned this from a reading list that felt haunted. Items I had dismissed came back after a refresh. Items I had finished came back as new, because the server treated "not in the inbox" as a single bit and then forgot which door they had left through. The cache was fast. It was faithfully wrong.

## Who owns what

The rule I use now is dull and strict. The client is the owner of queue and read state. The server is the owner of the document. A feed fetch may add items. It may not resurrect a dismissal, and it may not clear a \`readAt\` that the reader set on a train last night.

That split has a cost. You cannot just recompute the inbox from the source of truth, because the source of truth for attention lives on the device. Attention is not a property of the URL. Two people can save the same essay and owe it a completely different kind of time.

## Invalidation

Invalidation gets simpler once you admit this. Document changed: replace the body, keep the state. State changed: write the state, do not touch the body. If a write needs both, you are probably papering over a model that fused two lifetimes into one row.

\`\`\`
document -> replace body, keep state
state -> write state, keep body
\`\`\`

There is a place for a shared cache. Cover images, parsed HTML, the plain text you search across. Those are functions of the document. They can expire. The pencil mark in the margin cannot expire just because a CDN did.

> After the cache is emptied, a dismissed item stays dismissed, a finished essay stays findable, and a half-read piece in Later still opens where I stopped.

If a cache cannot pass that test, it is not a cache. It is a second, worse memory.`

const tracesMd = `The models got faster, and our notes got worse. We stored the answer and threw away the path. A month later the answer is still there, and nobody can say which paragraph it was answering.

I want a trace more than I want a summary. ${QUOTE_TRACE} The row will be stale by Thursday. The sentence is the thing I have to maintain.

## Claim, then evidence

When I review a change made with an agent, I read the diff the way I read a paper: claim, evidence, then the place the claim could fail. A trace that only says "updated the handler" is a shrug. A trace that quotes the failing line and the replacement is a margin note. I can argue with it.

This is also a storage decision. Tokens are cheap to count and expensive to reread. Traces are the opposite. They are slightly tedious to write and very fast to trust. I will pay the tedium. I will not pay the archaeology.

> A highlight with no note is a yellow stain. A highlight with one sentence in the margin is a future self, tapping the page.

There is a version of this for reading. Search should find that sentence. Search should not find the hundred headlines you skipped on the way to it.

\`\`\`
readAt: set
queue: untouched
\`\`\`

So the archive is small on purpose. Read means read. Everything else is a queue, a pencil mark, or a closed door. The trace of your attention is the product. The tokens were only the bill.`

const newsMd = `A lab released weights for a sparse mixture-of-experts model in the small-frontier class. On a public contest-math set it edges last year's best open model, with most of the gain on problems that need a short plan rather than a long calculation.

## What is actually interesting

The checkpoint is small enough to serve from a single high-memory workstation, and the expert routing is documented rather than implied. A short technical note describes which experts wake on formal proofs versus ordinary code.

## Two caveats

The math set has been public for a year, so contamination is a real question, not a footnote. And the human baseline they quote is a median contestant, not the top of the room.

If you only read one section, read the routing appendix. It is the rare release note that tells you when the model is unsure, instead of averaging that fact away.`

const importMd = `## Tactile models

A paper this week trains a world model on vision plus fingertip sense, and plans the next fraction of a second inside that model. The result will get written up as robots that feel. The narrower claim is better: when the hand hides the camera, touch carries the state that vision just lost.

## Decoding

A serving paper splits prefill from decode and lets expert replicas live where they are actually used. This is the kind of unglamorous systems work that changes what you can afford to run. Tail latency is the product. The architecture is how you stop paying prefill prices for a decode job.

\`\`\`
prefill pool: wide, bursty, throughput
decode pool: memory bandwidth, steady batches
\`\`\`

## Evals

No new leaderboard deserved your morning. One open sparse model posted a modest gain on contest math, with a routing appendix that is more useful than the score. Contamination remains the question that cheerful charts do not answer.

## To read slowly

> The latency you can feel is the latency that matters.

A list that paints late, or a state bit that confuses skipped with finished, will cost more trust than a model that is wrong in an obvious way.`

const batchMd = `## What changed

Two lines of work landed close together: world models that plan past the pretty next frame, and world models that take touch as seriously as pixels. Neither is a product. Both move the argument from "can it imagine a future" to "can a controller survive that future."

## Why it matters

A future you cannot plan with is a video. The new models are judged on pose, contact, and whether the subgoal is still reachable after the picture goes soft. That is a healthier score.

## What to ignore

Demos where a robot arm does one graceful thing in a clean room, scored by a caption. Wait for the failure table. Slippage, occlusion, and the long tail of worn hardware are the story.

> About ten seconds of latent rollout, supervised by pose and contact rather than by RGB. Long enough that a wrong model cannot hide.`

const tactileAbstract =
  'Robots that only predict pixels fail at the moment of contact. We train a world model on synchronized vision, joint torque, and fingertip tactile streams so that short-horizon plans can feel a slip before a camera can show it. The model factors the latent state into a slow scene code and a fast contact code. The contact code is trained to predict the next 200 milliseconds of tactile and torque signals, conditioned on a candidate action. At execution time a planner rolls out a few dozen action snippets inside the latent model and commits to the one whose predicted contact stays inside a friction cone estimated from the same latents. On connector insertion, cloth folding, and unscrewing a worn cap, the tactile model cuts slippage failures by about half relative to a vision-only world model of the same size. Gains are largest when the camera is briefly occluded by the hand, which is the ordinary case. We release the synchronized dataset and the planning loop. We do not claim a general household robot. We claim that contact has a representation, and that treating it as a footnote of vision is why the last centimeter still dominates the error.'

const horizonAbstract =
  'World models that predict the next frame look convincing and plan poorly. The error that matters is not pixel error. It is the drift of the quantities a controller actually uses: pose, contact, and whether the subgoal is still reachable. Horizon is a latent dynamics model trained to roll out those quantities for about ten seconds, past the horizon where RGB reconstruction remains sharp. We supervise the latent with probes for end-effector pose, object pose, and a binary contact flag, plus a contrastive loss that keeps reachable futures near the actions that produced them. Pixel reconstruction is an auxiliary loss with a low weight, used so the latent cannot ignore the scene. On long-horizon manipulation tasks the model plans with a simple shooting method and no separate high-level policy. It completes tasks whose successful execution takes four to eight seconds, where frame-predictive baselines collapse into visually plausible and physically useless futures. Remove the pose probes and long plans drift. Remove the pixel loss and the latent ignores distractors that later become the object in hand. Keep both, at the right weights, and the plan survives the frame where the picture goes soft.'

const splitAbstract =
  'Prefill and decode want different machines. Prefill is a wide, bursty matrix multiply that likes big batches and high throughput. Decode is a latency-bound walk that likes memory bandwidth and small, steady batches. Mixture-of-experts models make the split sharper, because each token wakes only a few experts, and the experts you need during prefill are not the experts you need one token later. Split Route places prefill and decode on separate pools and routes each expert replica to the pool where it earns its keep. A short scheduler moves hot experts toward the decode pool as a conversation settles and keeps cold experts available for the next prefill spike. The transfer is the weights, not the activations: a token KV cache stays on the decode worker that owns the sequence. Across traces from an internal assistant, the split cuts tail latency at the same cost, and cuts cost at the same tail latency, compared with a colocated baseline that sizes every GPU for the worse of the two phases. If the scheduler moves an expert too late, the first decoded tokens pay a fetch. We budget that fetch inside the same latency account as the network, and we refuse a move that would break it.'

function anchor(text: string, quote: string): { prefix: string; suffix: string } {
  const at = text.indexOf(quote)
  if (at < 0) throw new Error(`Seed quote missing: ${quote.slice(0, 32)}`)
  return {
    prefix: text.slice(Math.max(0, at - 32), at),
    suffix: text.slice(at + quote.length, at + quote.length + 32),
  }
}

function unread(): UserItemState {
  return { queue: 'firehose', readAt: null, readProgress: 0, dismissedAt: null }
}

const cacheAnchor = anchor(latencyMd, QUOTE_CACHE)
const queueAnchor = anchor(latencyMd, QUOTE_QUEUE)
const traceAnchor = anchor(tracesMd, QUOTE_TRACE)

const sources: Source[] = [
  {
    id: 'src_papers',
    name: 'ML Digest Papers',
    sourceType: 'ml_digest_paper',
    folder: 'Papers',
    defaultOpenMode: 'reader',
    defaultQueue: 'firehose',
    enabled: true,
  },
  {
    id: 'src_mlnews',
    name: 'ML Digest News',
    sourceType: 'ml_digest_news',
    folder: 'Briefings',
    defaultOpenMode: 'reader',
    defaultQueue: 'firehose',
    enabled: true,
  },
  {
    id: 'src_harbor',
    name: 'Harbor Notes',
    sourceType: 'rss',
    folder: 'Blogs',
    defaultOpenMode: 'reader',
    defaultQueue: 'firehose',
    feedUrl: 'https://harbor.example/feed.xml',
    enabled: true,
  },
  {
    id: 'src_kernel',
    name: 'Kernel Log',
    sourceType: 'rss',
    folder: 'Blogs',
    defaultOpenMode: 'reader',
    defaultQueue: 'firehose',
    feedUrl: 'https://kernel-log.example/rss',
    enabled: true,
  },
  {
    id: 'src_margin',
    name: 'Margin Journal',
    sourceType: 'rss',
    folder: 'Blogs',
    defaultOpenMode: 'reader',
    defaultQueue: 'firehose',
    feedUrl: 'https://margin.example/feed',
    enabled: true,
  },
  {
    id: 'src_import',
    name: 'Import AI',
    sourceType: 'newsletter',
    folder: 'Newsletters',
    defaultOpenMode: 'reader',
    defaultQueue: 'firehose',
    enabled: true,
  },
  {
    id: 'src_batch',
    name: 'The Batch',
    sourceType: 'newsletter',
    folder: 'Newsletters',
    defaultOpenMode: 'reader',
    defaultQueue: 'firehose',
    enabled: true,
  },
  {
    id: 'src_hn',
    name: 'Hacker News',
    sourceType: 'hacker_news',
    folder: 'HN',
    defaultOpenMode: 'external',
    defaultQueue: 'firehose',
    feedUrl: 'https://news.ycombinator.com',
    enabled: true,
  },
  {
    id: 'src_yt',
    name: 'Andrej Karpathy',
    sourceType: 'youtube',
    folder: 'Video',
    defaultOpenMode: 'external',
    defaultQueue: 'firehose',
    feedUrl: 'https://www.youtube.com/@AndrejKarpathy',
    enabled: true,
  },
  {
    id: 'src_pod',
    name: 'Latent Space',
    sourceType: 'podcast',
    folder: 'Podcasts',
    defaultOpenMode: 'deeplink',
    defaultQueue: 'firehose',
    feedUrl: 'https://www.latent.space/feed',
    enabled: true,
  },
  {
    id: 'src_wiki',
    name: 'Wikipedia Current Events',
    sourceType: 'web_snapshot',
    folder: 'Briefings',
    defaultOpenMode: 'external',
    defaultQueue: 'firehose',
    feedUrl: 'https://en.wikipedia.org/wiki/Portal:Current_events',
    enabled: true,
  },
  {
    id: 'src_chrome',
    name: 'Chrome',
    sourceType: 'chrome_ext',
    folder: 'Blogs',
    defaultOpenMode: 'external',
    defaultQueue: 'later',
    enabled: true,
  },
]

const items: Item[] = [
  {
    id: 'item_tactile',
    sourceId: 'src_papers',
    externalId: '2511.18421',
    canonicalUrl: 'https://arxiv.org/abs/2511.18421',
    title: 'Tactile World Models for Contact-Rich Manipulation',
    author: 'Voss, Ito, Raman',
    publishedAt: '2026-11-25T15:00:00.000Z',
    openMode: 'reader',
    actionUrl: 'https://arxiv.org/abs/2511.18421',
    plainText: tactileAbstract,
    metadata: {
      kind: 'paper',
      authors: ['Lena Voss', 'Rafael Ito', 'Priya Raman'],
      affiliations: ['MIT CSAIL', 'ETH Zurich'],
      arxivId: '2511.18421',
      abstract: tactileAbstract,
      links: [
        { label: 'arXiv', url: 'https://arxiv.org/abs/2511.18421' },
        { label: 'Project', url: 'https://harbor.example/tactile-world-models' },
        { label: 'Announcement', url: 'https://harbor.example/notes/tactile' },
      ],
      discussionQuotes: [
        {
          speaker: 'Lena Voss',
          text: 'The friction cone is read out of the same latent the planner searches, so the plan and the safety check cannot disagree about the state.',
        },
        {
          speaker: 'Rafael Ito',
          text: 'Most of the win shows up when the fingers hide the socket. Vision-only models hallucinate a clean insertion because the last frame looked fine.',
        },
        {
          speaker: 'Priya Raman',
          text: 'We still fail on soft cloth when the contact patch is larger than the sensor. That is a sensor problem as much as a model problem.',
        },
      ],
    },
  },
  {
    id: 'item_horizon',
    sourceId: 'src_papers',
    externalId: '2511.09014',
    canonicalUrl: 'https://arxiv.org/abs/2511.09014',
    title: 'Horizon: Latent Dynamics that Plan Past the Visible Frame',
    author: 'Adler, Zhou, Okonkwo',
    publishedAt: '2026-11-22T15:00:00.000Z',
    openMode: 'reader',
    actionUrl: 'https://arxiv.org/abs/2511.09014',
    plainText: horizonAbstract,
    metadata: {
      kind: 'paper',
      authors: ['Jonah Adler', 'Mei-Ling Zhou', 'Samuel Okonkwo'],
      affiliations: ['Stanford IRIS', 'UC Berkeley'],
      arxivId: '2511.09014',
      abstract: horizonAbstract,
      links: [
        { label: 'arXiv', url: 'https://arxiv.org/abs/2511.09014' },
        { label: 'Project', url: 'https://harbor.example/horizon' },
        { label: 'Announcement', url: 'https://harbor.example/notes/horizon' },
      ],
      discussionQuotes: [
        {
          speaker: 'Jonah Adler',
          text: 'Pixel loss is a regularizer. If you let it dominate, you get a pretty video model that cannot keep a pose.',
        },
        {
          speaker: 'Mei-Ling Zhou',
          text: 'Ten seconds is not a magic number. It is long enough that a wrong dynamics model cannot hide behind a sharp next frame.',
        },
        {
          speaker: 'Samuel Okonkwo',
          text: 'Shooting in latent space is boring on purpose. If a fancy planner is required, we do not yet trust the model.',
        },
      ],
    },
  },
  {
    id: 'item_split',
    sourceId: 'src_papers',
    externalId: '2510.22108',
    canonicalUrl: 'https://arxiv.org/abs/2510.22108',
    title: 'Split Route: Disaggregated Prefill and Decode for Mixture-of-Experts Serving',
    author: 'Shah, Berger, Ito',
    publishedAt: '2026-11-18T15:00:00.000Z',
    openMode: 'reader',
    actionUrl: 'https://arxiv.org/abs/2510.22108',
    plainText: splitAbstract,
    metadata: {
      kind: 'paper',
      authors: ['Anika Shah', 'Tom Berger', 'Hanae Ito'],
      affiliations: ['University of Washington', 'Northline Systems'],
      arxivId: '2510.22108',
      abstract: splitAbstract,
      links: [
        { label: 'arXiv', url: 'https://arxiv.org/abs/2510.22108' },
        { label: 'Project', url: 'https://northline.example/split-route' },
        { label: 'Announcement', url: 'https://northline.example/blog/split-route' },
      ],
      discussionQuotes: [
        {
          speaker: 'Anika Shah',
          text: 'Colocation looks simpler until you price the idle memory on a prefill-sized GPU that is now emitting one token at a time.',
        },
        {
          speaker: 'Tom Berger',
          text: 'The KV cache does not move. Once we moved caches we recreated the latency problem we were trying to leave behind.',
        },
        {
          speaker: 'Hanae Ito',
          text: 'Expert heat is stable over minutes and noisy over milliseconds. The scheduler should be almost bored.',
        },
      ],
    },
  },
  {
    id: 'item_sparse',
    sourceId: 'src_mlnews',
    externalId: 'ml-news-sparse-2026-11-26',
    canonicalUrl: 'https://digest.example/news/sparse-contest-math',
    title: 'Open sparse model posts a quiet gain on contest math',
    author: 'ML Digest',
    publishedAt: '2026-11-26T12:00:00.000Z',
    openMode: 'reader',
    actionUrl: 'https://digest.example/news/sparse-contest-math',
    contentMarkdown: newsMd,
    plainText: newsMd,
    metadata: { kind: 'generic' },
  },
  {
    id: 'item_latency',
    sourceId: 'src_harbor',
    externalId: 'harbor-latency-budget',
    canonicalUrl: 'https://harbor.example/latency-budget',
    title: 'The Latency Budget',
    author: 'Mira Adelman',
    publishedAt: '2026-11-20T15:00:00.000Z',
    openMode: 'reader',
    actionUrl: 'https://harbor.example/latency-budget',
    contentMarkdown: latencyMd,
    plainText: latencyMd,
    metadata: { kind: 'generic' },
  },
  {
    id: 'item_caches',
    sourceId: 'src_kernel',
    externalId: 'kernel-caches-product',
    canonicalUrl: 'https://kernel-log.example/caches-are-a-product-decision',
    title: 'Caches Are a Product Decision',
    author: 'Evan Cho',
    publishedAt: '2026-11-21T15:00:00.000Z',
    openMode: 'reader',
    actionUrl: 'https://kernel-log.example/caches-are-a-product-decision',
    contentMarkdown: cachesMd,
    plainText: cachesMd,
    metadata: { kind: 'generic' },
  },
  {
    id: 'item_traces',
    sourceId: 'src_margin',
    externalId: 'margin-traces-not-tokens',
    canonicalUrl: 'https://margin.example/traces-not-tokens',
    title: 'Traces, Not Tokens',
    author: 'Nadia Rahman',
    publishedAt: '2026-11-12T15:00:00.000Z',
    openMode: 'reader',
    actionUrl: 'https://margin.example/traces-not-tokens',
    contentMarkdown: tracesMd,
    plainText: tracesMd,
    metadata: { kind: 'generic' },
  },
  {
    id: 'item_import',
    sourceId: 'src_import',
    externalId: 'import-ai-412',
    canonicalUrl: 'https://importai.example/412',
    title: 'Import AI 412: tactile models, cheaper decoding, a quiet week in evals',
    author: 'Import AI',
    publishedAt: '2026-11-24T15:00:00.000Z',
    openMode: 'reader',
    actionUrl: 'https://importai.example/412',
    contentMarkdown: importMd,
    plainText: importMd,
    metadata: { kind: 'generic' },
  },
  {
    id: 'item_batch',
    sourceId: 'src_batch',
    externalId: 'batch-world-models-bench',
    canonicalUrl: 'https://batch.example/world-models-leave-the-lab-bench',
    title: 'World models leave the lab bench',
    author: 'The Batch',
    publishedAt: '2026-11-23T15:00:00.000Z',
    openMode: 'reader',
    actionUrl: 'https://batch.example/world-models-leave-the-lab-bench',
    contentMarkdown: batchMd,
    plainText: batchMd,
    metadata: { kind: 'generic' },
  },
  {
    id: 'item_sqlite',
    sourceId: 'src_hn',
    externalId: 'hn-42111890',
    canonicalUrl: 'https://rrau.example/sqlite-wal-reading-archive',
    discussionUrl: 'https://news.ycombinator.com/item?id=42111890',
    title: 'SQLite WAL mode is enough for a personal reading archive',
    author: 'rrau',
    publishedAt: '2026-11-26T16:10:00.000Z',
    openMode: 'external',
    actionUrl: 'https://rrau.example/sqlite-wal-reading-archive',
    plainText:
      'A short argument that one local SQLite file, in WAL mode, can hold items, read state, and highlights for a personal reader without a server in the loop.',
    metadata: { kind: 'hn', points: 286, commentCount: 143 },
  },
  {
    id: 'item_skipped',
    sourceId: 'src_hn',
    externalId: 'hn-42088411',
    canonicalUrl: 'https://tormund.example/skipped-is-not-finished',
    discussionUrl: 'https://news.ycombinator.com/item?id=42088411',
    title: 'Read-it-later apps keep confusing skipped with finished',
    author: 'tormund',
    publishedAt: '2026-11-19T18:40:00.000Z',
    openMode: 'external',
    actionUrl: 'https://tormund.example/skipped-is-not-finished',
    plainText:
      'Notes on queue design. Dismiss should leave the inbox and stay out of the archive that search reads.',
    metadata: { kind: 'hn', points: 94, commentCount: 61 },
  },
  {
    id: 'item_gpt',
    sourceId: 'src_yt',
    externalId: 'yt-kCc8FmEb1nY',
    canonicalUrl: 'https://www.youtube.com/watch?v=kCc8FmEb1nY',
    title: "Let's build GPT: from scratch, in code, spelled out",
    author: 'Andrej Karpathy',
    publishedAt: '2026-11-15T15:00:00.000Z',
    openMode: 'external',
    actionUrl: 'https://www.youtube.com/watch?v=kCc8FmEb1nY',
    plainText:
      'A full walkthrough of a GPT, from a blank file to a trained model, for people who want the mechanics rather than the metaphor.',
    metadata: { kind: 'youtube', channelName: 'Andrej Karpathy', durationMin: 116 },
  },
  {
    id: 'item_contact',
    sourceId: 'src_pod',
    externalId: 'ls-contact-1844',
    canonicalUrl: 'https://www.latent.space/p/contact-world-models',
    title: 'Contact, world models, and the last centimeter',
    author: 'Latent Space',
    publishedAt: '2026-11-17T15:00:00.000Z',
    openMode: 'deeplink',
    actionUrl: 'overcast://x-callback-url/open?url=https%3A%2F%2Fwww.latent.space%2Fp%2Fcontact-world-models',
    plainText:
      'A conversation about tactile world models, what the benchmarks hide, and why the last centimeter of contact still dominates robot error.',
    metadata: { kind: 'podcast', durationMin: 48, showName: 'Latent Space' },
  },
  {
    id: 'item_wiki',
    sourceId: 'src_wiki',
    externalId: 'wiki-current-events',
    canonicalUrl: 'https://en.wikipedia.org/wiki/Portal:Current_events',
    title: 'Portal: Current events',
    author: 'Wikipedia',
    publishedAt: '2026-11-27T06:00:00.000Z',
    openMode: 'external',
    actionUrl: 'https://en.wikipedia.org/wiki/Portal:Current_events',
    plainText: 'Daily snapshot of the Wikipedia current events portal.',
    metadata: { kind: 'generic' },
  },
  {
    id: 'item_localfirst',
    sourceId: 'src_chrome',
    externalId: 'https://www.inkandswitch.com/essay/local-first/',
    canonicalUrl: 'https://www.inkandswitch.com/essay/local-first/',
    title: 'Local-first software',
    author: 'Ink & Switch',
    publishedAt: '2026-11-10T15:00:00.000Z',
    openMode: 'external',
    actionUrl: 'https://www.inkandswitch.com/essay/local-first/',
    plainText:
      'Seven ideals for software that keeps the device as the source of truth, including the ability to work offline and to keep data under the owner.',
    metadata: { kind: 'generic' },
  },
]

const states: Record<string, UserItemState> = {
  item_tactile: unread(),
  item_horizon: unread(),
  item_split: unread(),
  item_sparse: unread(),
  item_latency: { queue: 'later', readAt: null, readProgress: 0.35, dismissedAt: null },
  item_caches: unread(),
  item_traces: { queue: 'firehose', readAt: '2026-11-24T21:05:00.000Z', readProgress: 1, dismissedAt: null },
  item_import: unread(),
  item_batch: unread(),
  item_sqlite: unread(),
  item_skipped: unread(),
  item_gpt: unread(),
  item_contact: unread(),
  item_wiki: unread(),
  item_localfirst: { queue: 'none', readAt: '2026-11-25T18:12:00.000Z', readProgress: 1, dismissedAt: null },
}

const highlights: Highlight[] = [
  {
    id: 'hl_cache',
    itemId: 'item_latency',
    quote: QUOTE_CACHE,
    prefix: cacheAnchor.prefix,
    suffix: cacheAnchor.suffix,
    note: 'This is the product claim, not the profiler claim.',
    createdAt: '2026-11-21T15:10:00.000Z',
    updatedAt: '2026-11-21T15:10:00.000Z',
  },
  {
    id: 'hl_queue',
    itemId: 'item_latency',
    quote: QUOTE_QUEUE,
    prefix: queueAnchor.prefix,
    suffix: queueAnchor.suffix,
    note: 'Orthogonal state. Dismiss must not become read.',
    createdAt: '2026-11-21T15:14:00.000Z',
    updatedAt: '2026-11-21T15:14:00.000Z',
  },
  {
    id: 'hl_trace',
    itemId: 'item_traces',
    quote: QUOTE_TRACE,
    prefix: traceAnchor.prefix,
    suffix: traceAnchor.suffix,
    note: 'Keep this next to the eval section when I rewrite it.',
    createdAt: '2026-11-24T21:02:00.000Z',
    updatedAt: '2026-11-24T21:02:00.000Z',
  },
]

export function createSeed(): Library {
  return {
    sources: structuredClone(sources),
    items: structuredClone(items),
    states: structuredClone(states),
    highlights: structuredClone(highlights),
  }
}
