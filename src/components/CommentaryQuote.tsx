export interface CommentaryQuoteProps {
  text: string
  name: string
  detail?: string
  href?: string
}

export function CommentaryQuote({ text, name, detail, href }: CommentaryQuoteProps) {
  return (
    <figure className="commentary-quote">
      <p className="commentary-text">{text}</p>
      <figcaption>
        {href ? (
          <a className="commentary-name" href={href} target="_blank" rel="noopener noreferrer">
            {name}
          </a>
        ) : (
          <span className="commentary-name">{name}</span>
        )}
        {detail ? <span className="commentary-detail">{detail}</span> : null}
      </figcaption>
    </figure>
  )
}

export function CommentaryList({
  quotes,
}: {
  quotes: { text: string; name: string; detail?: string; href?: string }[]
}) {
  return (
    <div className="commentary">
      {quotes.map((quote) => (
        <CommentaryQuote key={`${quote.name}:${quote.text}`} {...quote} />
      ))}
    </div>
  )
}
