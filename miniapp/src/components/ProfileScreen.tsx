...
const handleDonateCompleted = useCallback(
  (amount: number, currency: string) => {
    setDonateThanks(true);
    setDonateAmount(amount);
    setDonateCurrency(currency);
  },
  [],
);

const handleDonateThanksDone = useCallback(() => {
  setDonateThanks(false);
  setDonateAmount(undefined);
  setDonateCurrency(undefined);
}, []);

...

{donateThanks && (
  <DonateThanksToast
    onDone={handleDonateThanksDone}
    amount={donateAmount}
    currency={donateCurrency}
  />
)}
...