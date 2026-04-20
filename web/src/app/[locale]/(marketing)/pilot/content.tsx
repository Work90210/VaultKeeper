'use client';

import { useState } from 'react';

export function PilotForm() {
  const [submitted, setSubmitted] = useState(false);

  function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setSubmitted(true);
  }

  return (
    <form className="pilot-form" onSubmit={handleSubmit}>
      <div className="fhead">
        <h3>Pilot request &mdash; <em>VK-PILOT-2026</em></h3>
        <span>encrypted in transit &middot; PGP: 3F9A&hellip;</span>
      </div>
      <div className="row">
        <div><label>Institution</label><input placeholder="e.g. Kosovo Specialist Chambers" required /></div>
        <div><label>Jurisdiction</label><input placeholder="Country / sovereignty tier" required /></div>
      </div>
      <div className="row">
        <div><label>Your role</label><input placeholder="Head of IT &middot; Registrar &middot; Legal" required /></div>
        <div>
          <label>Team size</label>
          <select>
            <option>3&ndash;15 (NGO / unit)</option>
            <option>16&ndash;50 (mid-tier chamber)</option>
            <option>51&ndash;150 (tribunal)</option>
            <option>150+ (ICC-scale)</option>
          </select>
        </div>
      </div>
      <div className="row">
        <div><label>Current system</label><input placeholder="Relativity &middot; Nuix &middot; Google Drive &middot; none" /></div>
        <div><label>Approx. evidence volume</label><input placeholder="~ 8 TB &middot; 40 TB &middot; PB" /></div>
      </div>
      <div className="row" style={{ gridTemplateColumns: '1fr' }}>
        <div><label>What would make the pilot a success for you?</label><textarea placeholder="e.g. &lsquo;A sealed export the ICC Registrar&rsquo;s office will accept on first submission.&rsquo;"></textarea></div>
      </div>
      <div className="submit">
        <span className="note">We&rsquo;ll respond within 48 hours with two timeslot options and a short pre-read. No follow-up automation, no CRM chase.</span>
        <button className="btn" type="submit" disabled={submitted}>
          {submitted ? 'Sent \u2713' : <>Request pilot <span className="arr">&rarr;</span></>}
        </button>
      </div>
    </form>
  );
}
