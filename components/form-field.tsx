"use client";
export function FormField({ label, name, required = false, type = 'text' }: { label: string; name: string; required?: boolean; type?: string; }) {
  return (
    <div className='mb-4 flex flex-col'>
      <label htmlFor={name} className='font-open_sans font-bold text-sm inline-block float-left'>
        {label}
        {required && <span className='text-xs text-tiger'> *</span>}
      </label>
      {type === 'textarea' ?
        <textarea className='px-3 py-2 text-sm text-cornsilk dark:text-tiger rounded-md bg-earth dark:bg-cornsilk' /> :
        <input type={type} id={name} name={name} className='px-3 py-2 text-sm text-cornsilk dark:text-tiger rounded-md bg-earth dark:bg-cornsilk' aria-label={name} required={required} />}
    </div>
  );
}
