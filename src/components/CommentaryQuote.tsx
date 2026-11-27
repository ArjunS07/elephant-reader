import type { DiscussionQuote, QuoteReply } from '../types.ts'
import { safeUrl } from '../lib/format.ts'
import { kindLabel, speakerAffiliation } from '../lib/quotes.ts'

function Reply({ reply }: { reply: QuoteReply }) {
  const href = safeUrl(reply.url)
  return (
    <div className="commentary-reply">
      <p className="commentary-text">{reply.text}</p>
      {reply.traction || href ? (
        <p className="commentary-meta">
          {href ? (
            <a href={href} target="_blank" rel="noopener noreferrer">
              {reply.traction ?? 'Link'}
            </a>
          ) : (
            reply.traction
          )}
        </p>
      ) : null}
    </div>
  )
}

export function CommentaryQuote({ quote }: { quote: DiscussionQuote }) {
  const affiliation = speakerAffiliation(quote)
  const kind = kindLabel(quote.kind)
  const href = safeUrl(quote.url)
  return (
    <figure className="commentary-quote">
      <p className="commentary-text">{quote.text}</p>
      <figcaption>
        {href ? (
          <a className="commentary-name" href={href} target="_blank" rel="noopener noreferrer">
            {quote.speaker}
          </a>
        ) : (
          <span className="commentary-name">{quote.speaker}</span>
        )}
        {affiliation ? <span className="commentary-detail">{affiliation}</span> : null}
        {kind ? <span className="commentary-cue">{kind}</span> : null}
        {quote.traction ? <span className="commentary-cue">{quote.traction}</span> : null}
      </figcaption>
      {quote.replies?.length ? (
        <div className="commentary-replies">
          {quote.replies.map((reply) => (
            <Reply key={reply.text} reply={reply} />
          ))}
        </div>
      ) : null}
    </figure>
  )
}

export function CommentaryList({ quotes }: { quotes: DiscussionQuote[] }) {
  if (!quotes.length) return null
  return (
    <div className="commentary">
      {quotes.map((quote) => (
        <CommentaryQuote key={`${quote.speaker}:${quote.text}`} quote={quote} />
      ))}
    </div>
  )
}
