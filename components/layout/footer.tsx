import Link from 'next/link'
import resumeData from '../../lib/resumeData.json';
import { useMemo } from 'react';

interface Network {
  network: {
    url: string,
    className: string,
    name: string
  }
};

function NetworkLink({ network }: Network) {
  const { className, name, url } = network;
  return <li>
    <Link href={url} title={name}>
      <i className={className}></i>
    </Link>
  </li>;
}

export default function Footer({ home }: { home?: boolean }) {
  const networks = useMemo(() => {
    return resumeData.main.social.map((network) => <NetworkLink key={network.name} network={network} />);
  }, []);

  return (
    <footer>
      <div className="row">
        <div className="twelve columns">
          <ul className="social-links">
            {networks}
          </ul>
        </div>
        {home && <div id="go-top"><a className="smoothscroll" title="Back to Top" href="#home"><i className="icon-up-open"></i></a></div>}
      </div>
    </footer>
  );
}
