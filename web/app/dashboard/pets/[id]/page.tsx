import { PetForm } from "@/components/pet-form";

export default function PetDetailPage({ params }: { params: { id: string } }) {
  return <PetForm mode="edit" petId={params.id} />;
}
