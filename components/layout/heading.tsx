export default function Heading({ title }: { title: string }) {
    return (
        <div className="mt-5 pt-2 relative min-h-[1px] w-auto">
            <h1 className="uppercase tracking-widest text-2xl font-bold text-center">
                <span className="border-b-2 border-green-800 pb-1">
                    {title}
                </span>
            </h1>
        </div>
    );
}